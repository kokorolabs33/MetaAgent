// Package main implements LLM-powered Deal Review agents as A2A servers.
// A single binary serves one of 4 roles (legal, finance, technical, deal-review)
// selected via the --role flag. Each role has a distinct system prompt and calls
// the Claude Messages API to generate structured analysis.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// ---- A2A Protocol Types (same as mockagent) ----

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	ID      string          `json:"id"`
	Params  json.RawMessage `json:"params"`
}

type jsonRPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      string    `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type sendMessageParams struct {
	Message   a2aMessage `json:"message"`
	ContextID string     `json:"contextId,omitempty"`
	TaskID    string     `json:"taskId,omitempty"`
}

type taskIDParams struct {
	ID string `json:"id"`
}

type a2aMessage struct {
	Role  string        `json:"role"`
	Parts []messagePart `json:"parts"`
}

type messagePart struct {
	Text string `json:"text,omitempty"`
	Data any    `json:"data,omitempty"`
}

type a2aTask struct {
	ID        string     `json:"id"`
	ContextID string     `json:"contextId,omitempty"`
	Status    a2aStatus  `json:"status"`
	Artifacts []artifact `json:"artifacts,omitempty"`
}

type a2aStatus struct {
	State   string      `json:"state"`
	Message *a2aMessage `json:"message,omitempty"`
}

type artifact struct {
	ArtifactID string        `json:"artifactId,omitempty"`
	Parts      []messagePart `json:"parts,omitempty"`
}

type agentCard struct {
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	URL                string          `json:"url"`
	Version            string          `json:"version"`
	Capabilities       map[string]bool `json:"capabilities"`
	Skills             []cardSkill     `json:"skills"`
	DefaultInputModes  []string        `json:"defaultInputModes"`
	DefaultOutputModes []string        `json:"defaultOutputModes"`
}

type cardSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ---- Task State ----

type taskState struct {
	mu        sync.Mutex
	ID        string
	ContextID string
	Status    string // "working", "completed", "failed", "input-required"
	Result    string
	Error     string
}

var (
	tasks    sync.Map
	executor *LLMExecutor
)

func main() {
	roleFlag := flag.String("role", "", "agent role: legal, finance, technical, deal-review")
	port := flag.Int("port", 9091, "listen port")
	flag.Parse()

	if *roleFlag == "" {
		fmt.Fprintf(os.Stderr, "Usage: llmagent --role=<legal|finance|technical|deal-review> [--port=9091]\n")
		os.Exit(1)
	}

	role, ok := roles[*roleFlag]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown role: %s\nAvailable: legal, finance, technical, deal-review\n", *roleFlag)
		os.Exit(1)
	}

	claude := NewClaudeClient()
	if claude.APIKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required")
	}

	executor = NewLLMExecutor(role, claude)

	baseURL := fmt.Sprintf("http://localhost:%d", *port)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// A2A AgentCard discovery
	r.Get("/.well-known/agent-card.json", func(w http.ResponseWriter, _ *http.Request) {
		card := agentCard{
			Name:        role.Name,
			Description: role.Description,
			URL:         baseURL,
			Version:     "1.0.0",
			Capabilities: map[string]bool{
				"streaming":              false,
				"pushNotifications":      false,
				"stateTransitionHistory": false,
			},
			Skills: []cardSkill{
				{ID: role.SkillID, Name: role.SkillName, Description: role.Description},
			},
			DefaultInputModes:  []string{"text/plain", "application/json"},
			DefaultOutputModes: []string{"application/json"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(card)
	})

	// A2A JSON-RPC endpoint
	r.Post("/", handleJSONRPC)

	// Health endpoint
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("%s listening on %s (model: %s)", role.Name, addr, claude.Model)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	var req jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPCError(w, "", -32700, "Parse error")
		return
	}

	ctx := r.Context()

	switch req.Method {
	case "message/send":
		handleSendMessage(ctx, w, req)
	case "tasks/get":
		handleGetTask(w, req)
	case "tasks/cancel":
		handleCancelTask(w, req)
	default:
		writeRPCError(w, req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func handleSendMessage(ctx context.Context, w http.ResponseWriter, req jsonRPCRequest) {
	var params sendMessageParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "Invalid params")
		return
	}

	// Extract text and data parts from the message
	var text string
	var data map[string]any
	for _, part := range params.Message.Parts {
		if part.Text != "" {
			text = part.Text
		}
		if part.Data != nil {
			// Data parts carry upstream analysis results
			if m, ok := part.Data.(map[string]any); ok {
				data = m
			}
		}
	}

	contextID := params.ContextID
	if contextID == "" {
		contextID = uuid.New().String()
	}

	// Check if this is a follow-up to an existing task (input-required response)
	if params.TaskID != "" {
		if val, ok := tasks.Load(params.TaskID); ok {
			ts := val.(*taskState)
			ts.mu.Lock()

			if ts.Status == "input-required" {
				ts.mu.Unlock()

				// Process follow-up through the LLM
				response, err := executor.HandleFollowUp(ctx, ts.ContextID, text)
				if err != nil {
					log.Printf("follow-up error: %v", err)
					ts.mu.Lock()
					ts.Status = "failed"
					ts.Error = err.Error()
					ts.mu.Unlock()
					cleanupTask(ts.ID, ts.ContextID, executor)

					writeRPCResult(w, req.ID, a2aTask{
						ID:        ts.ID,
						ContextID: ts.ContextID,
						Status: a2aStatus{
							State: "failed",
							Message: &a2aMessage{
								Role:  "agent",
								Parts: []messagePart{{Text: err.Error()}},
							},
						},
					})
					return
				}

				ts.mu.Lock()
				ts.Status = "completed"
				ts.Result = response
				ts.mu.Unlock()
				cleanupTask(ts.ID, ts.ContextID, executor)

				writeRPCResult(w, req.ID, a2aTask{
					ID:        ts.ID,
					ContextID: ts.ContextID,
					Status:    a2aStatus{State: "completed"},
					Artifacts: []artifact{{
						ArtifactID: uuid.New().String(),
						Parts:      []messagePart{{Text: response}},
					}},
				})
				return
			}
			ts.mu.Unlock()
		}
	}

	// New task — process through the LLM
	taskID := uuid.New().String()
	ts := &taskState{
		ID:        taskID,
		ContextID: contextID,
		Status:    "working",
	}
	tasks.Store(taskID, ts)

	response, inputRequired, inputMessage, err := executor.HandleMessage(
		ctx, contextID, text, data,
	)
	if err != nil {
		log.Printf("LLM error: %v", err)
		ts.mu.Lock()
		ts.Status = "failed"
		ts.Error = err.Error()
		ts.mu.Unlock()
		cleanupTask(taskID, contextID, executor)

		writeRPCResult(w, req.ID, a2aTask{
			ID:        taskID,
			ContextID: contextID,
			Status: a2aStatus{
				State: "failed",
				Message: &a2aMessage{
					Role:  "agent",
					Parts: []messagePart{{Text: err.Error()}},
				},
			},
		})
		return
	}

	if inputRequired {
		ts.mu.Lock()
		ts.Status = "input-required"
		ts.Result = response
		ts.mu.Unlock()

		writeRPCResult(w, req.ID, a2aTask{
			ID:        taskID,
			ContextID: contextID,
			Status: a2aStatus{
				State: "input-required",
				Message: &a2aMessage{
					Role:  "agent",
					Parts: []messagePart{{Text: inputMessage}},
				},
			},
			Artifacts: []artifact{{
				ArtifactID: uuid.New().String(),
				Parts:      []messagePart{{Text: response}},
			}},
		})
		return
	}

	// Completed successfully
	ts.mu.Lock()
	ts.Status = "completed"
	ts.Result = response
	ts.mu.Unlock()
	cleanupTask(taskID, contextID, executor)

	writeRPCResult(w, req.ID, a2aTask{
		ID:        taskID,
		ContextID: contextID,
		Status:    a2aStatus{State: "completed"},
		Artifacts: []artifact{{
			ArtifactID: uuid.New().String(),
			Parts:      []messagePart{{Text: response}},
		}},
	})
}

func handleGetTask(w http.ResponseWriter, req jsonRPCRequest) {
	var params taskIDParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "Invalid params")
		return
	}

	val, ok := tasks.Load(params.ID)
	if !ok {
		writeRPCError(w, req.ID, -32001, "Task not found")
		return
	}

	ts := val.(*taskState)
	ts.mu.Lock()
	defer ts.mu.Unlock()

	task := a2aTask{
		ID:        ts.ID,
		ContextID: ts.ContextID,
		Status:    a2aStatus{State: ts.Status},
	}

	if ts.Status == "completed" && ts.Result != "" {
		task.Artifacts = []artifact{{
			ArtifactID: uuid.New().String(),
			Parts:      []messagePart{{Text: ts.Result}},
		}}
	}

	if ts.Status == "failed" && ts.Error != "" {
		task.Status.Message = &a2aMessage{
			Role:  "agent",
			Parts: []messagePart{{Text: ts.Error}},
		}
	}

	if ts.Status == "input-required" {
		task.Status.Message = &a2aMessage{
			Role:  "agent",
			Parts: []messagePart{{Text: "Waiting for input"}},
		}
		if ts.Result != "" {
			task.Artifacts = []artifact{{
				ArtifactID: uuid.New().String(),
				Parts:      []messagePart{{Text: ts.Result}},
			}}
		}
	}

	writeRPCResult(w, req.ID, task)
}

func handleCancelTask(w http.ResponseWriter, req jsonRPCRequest) {
	var params taskIDParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "Invalid params")
		return
	}

	val, ok := tasks.Load(params.ID)
	if !ok {
		writeRPCError(w, req.ID, -32001, "Task not found")
		return
	}

	ts := val.(*taskState)
	ts.mu.Lock()
	ts.Status = "canceled"
	ts.mu.Unlock()
	cleanupTask(ts.ID, ts.ContextID, executor)

	writeRPCResult(w, req.ID, a2aTask{
		ID:        ts.ID,
		ContextID: ts.ContextID,
		Status:    a2aStatus{State: "canceled"},
	})
}

func writeRPCResult(w http.ResponseWriter, id string, result any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func writeRPCError(w http.ResponseWriter, id string, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	})
}

// cleanupTask schedules removal of a completed/failed/canceled task and its
// conversation history after a grace period, preventing unbounded map growth.
func cleanupTask(taskID string, contextID string, ex *LLMExecutor) {
	time.AfterFunc(5*time.Minute, func() {
		tasks.Delete(taskID)
		ex.CleanupContext(contextID)
	})
}
