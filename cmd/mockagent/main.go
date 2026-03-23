// Package main implements a mock A2A agent server for testing.
// It supports various behavior keywords in the instruction text to simulate different
// agent scenarios (echo, slow, fail, input-required, etc.).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// ---- A2A Protocol Types ----

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
	mu          sync.Mutex
	ID          string
	ContextID   string
	Status      string // "working", "completed", "failed", "input-required"
	Instruction string
	Result      string
	Error       string
}

var tasks sync.Map

func main() {
	port := flag.Int("port", 9090, "listen port")
	name := flag.String("name", "Mock Agent", "agent name")
	flag.Parse()

	baseURL := fmt.Sprintf("http://localhost:%d", *port)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// A2A AgentCard discovery
	r.Get("/.well-known/agent-card.json", func(w http.ResponseWriter, _ *http.Request) {
		card := agentCard{
			Name:        *name,
			Description: "Mock agent for testing. Supports keyword-based behaviors.",
			URL:         baseURL,
			Version:     "1.0.0",
			Capabilities: map[string]bool{
				"streaming":              false,
				"pushNotifications":      false,
				"stateTransitionHistory": false,
			},
			Skills: []cardSkill{
				{ID: "mock", Name: "Mock Behavior", Description: "Simulates various agent behaviors via keywords"},
			},
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain", "application/json"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(card)
	})

	// A2A JSON-RPC endpoint
	r.Post("/", handleJSONRPC)

	// Legacy health endpoint
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Mock A2A agent %q listening on %s", *name, addr)
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

	switch req.Method {
	case "message/send":
		handleSendMessage(w, req)
	case "tasks/get":
		handleGetTask(w, req)
	case "tasks/cancel":
		handleCancelTask(w, req)
	default:
		writeRPCError(w, req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func handleSendMessage(w http.ResponseWriter, req jsonRPCRequest) {
	var params sendMessageParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "Invalid params")
		return
	}

	// Extract instruction text from message parts
	instruction := ""
	for _, part := range params.Message.Parts {
		if part.Text != "" {
			instruction = part.Text
			break
		}
	}

	// Check if this is a follow-up to an existing task
	if params.TaskID != "" {
		if val, ok := tasks.Load(params.TaskID); ok {
			ts := val.(*taskState)
			ts.mu.Lock()
			defer ts.mu.Unlock()

			// Follow-up message completes the task
			ts.Status = "completed"
			ts.Result = fmt.Sprintf("received follow-up: %s", instruction)

			writeRPCResult(w, req.ID, a2aTask{
				ID:        ts.ID,
				ContextID: ts.ContextID,
				Status:    a2aStatus{State: "completed"},
				Artifacts: []artifact{{
					ArtifactID: uuid.New().String(),
					Parts:      []messagePart{{Text: ts.Result}},
				}},
			})
			return
		}
	}

	// New task — determine behavior from instruction keywords
	taskID := uuid.New().String()
	ts := &taskState{
		ID:          taskID,
		ContextID:   params.ContextID,
		Instruction: instruction,
		Status:      "working",
	}

	switch {
	case strings.HasPrefix(instruction, "echo:"):
		msg := strings.TrimPrefix(instruction, "echo:")
		ts.Status = "completed"
		ts.Result = msg
		tasks.Store(taskID, ts)

		writeRPCResult(w, req.ID, a2aTask{
			ID:        taskID,
			ContextID: params.ContextID,
			Status:    a2aStatus{State: "completed"},
			Artifacts: []artifact{{
				ArtifactID: uuid.New().String(),
				Parts:      []messagePart{{Text: msg}},
			}},
		})
		return

	case strings.HasPrefix(instruction, "fail:"):
		msg := strings.TrimPrefix(instruction, "fail:")
		ts.Status = "failed"
		ts.Error = msg
		tasks.Store(taskID, ts)

		writeRPCResult(w, req.ID, a2aTask{
			ID:        taskID,
			ContextID: params.ContextID,
			Status: a2aStatus{
				State: "failed",
				Message: &a2aMessage{
					Role:  "agent",
					Parts: []messagePart{{Text: msg}},
				},
			},
		})
		return

	case strings.HasPrefix(instruction, "input:"):
		msg := strings.TrimPrefix(instruction, "input:")
		ts.Status = "input-required"
		tasks.Store(taskID, ts)

		writeRPCResult(w, req.ID, a2aTask{
			ID:        taskID,
			ContextID: params.ContextID,
			Status: a2aStatus{
				State: "input-required",
				Message: &a2aMessage{
					Role:  "agent",
					Parts: []messagePart{{Text: msg}},
				},
			},
		})
		return

	case strings.HasPrefix(instruction, "slow:"):
		n := parseIntSuffix(instruction, "slow:")

		// Simulate slow processing synchronously before returning
		time.Sleep(time.Duration(n) * time.Second)

		ts.Status = "completed"
		ts.Result = fmt.Sprintf("completed after %d seconds", n)
		tasks.Store(taskID, ts)

		writeRPCResult(w, req.ID, a2aTask{
			ID:        taskID,
			ContextID: params.ContextID,
			Status:    a2aStatus{State: "completed"},
			Artifacts: []artifact{{
				ArtifactID: uuid.New().String(),
				Parts:      []messagePart{{Text: fmt.Sprintf("completed after %d seconds delay", n)}},
			}},
		})
		return

	default:
		// Default: immediate completion
		ts.Status = "completed"
		ts.Result = "done"
		tasks.Store(taskID, ts)

		writeRPCResult(w, req.ID, a2aTask{
			ID:        taskID,
			ContextID: params.ContextID,
			Status:    a2aStatus{State: "completed"},
			Artifacts: []artifact{{
				ArtifactID: uuid.New().String(),
				Parts:      []messagePart{{Text: "done"}},
			}},
		})
		return
	}
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

// parseIntSuffix extracts the integer after a keyword prefix (e.g., "slow:3" -> 3).
// Returns 2 as default if parsing fails.
func parseIntSuffix(s, prefix string) int {
	v, err := strconv.Atoi(strings.TrimPrefix(s, prefix))
	if err != nil || v <= 0 {
		return 2
	}
	return v
}
