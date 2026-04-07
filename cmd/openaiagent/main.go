// Package main implements OpenAI-powered agents as A2A servers.
// A single binary serves one of 4 roles (engineering, finance, legal, marketing)
// selected via the --role flag. Each role has a distinct system prompt and calls
// the OpenAI Chat Completions API to generate responses.
//
// Protocol types are imported from taskhub/internal/a2a to avoid duplication.
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

	"taskhub/internal/a2a"
)

// taskState tracks in-flight tasks.
type taskState struct {
	mu           sync.Mutex
	ID           string
	ContextID    string
	Status       string // "working", "completed", "failed", "canceled", "input-required"
	Result       string
	Error        string
	ToolProgress string // Current tool call status for polling visibility
	CreatedAt    time.Time
	CompletedAt  time.Time
}

var (
	tasks         sync.Map
	conversations sync.Map // contextID -> []chatMessage
	client        *OpenAIClient
	role          Role
)

// getHistory returns the conversation history for a contextID.
func getHistory(contextID string) []chatMessage {
	val, ok := conversations.Load(contextID)
	if !ok {
		return nil
	}
	return val.([]chatMessage)
}

// appendHistory appends messages to the conversation history for a contextID,
// keeping at most 40 messages to accommodate tool call/result pairs.
func appendHistory(contextID string, msgs ...chatMessage) {
	existing := getHistory(contextID)
	updated := append(existing, msgs...)
	if len(updated) > 40 {
		updated = updated[len(updated)-40:]
	}
	conversations.Store(contextID, updated)
}

func main() {
	roleFlag := flag.String("role", "", "agent role: engineering, finance, legal, marketing")
	port := flag.Int("port", 0, "listen port (defaults to role's default port)")
	flag.Parse()

	if *roleFlag == "" {
		fmt.Fprintf(os.Stderr, "Usage: openaiagent --role=<engineering|finance|legal|marketing> [--port=PORT]\n")
		os.Exit(1)
	}

	var ok bool
	role, ok = roles[*roleFlag]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown role: %s\nAvailable: engineering, finance, legal, marketing\n", *roleFlag)
		os.Exit(1)
	}

	if *port == 0 {
		*port = role.DefaultPort
	}

	client = NewOpenAIClient()

	baseURL := fmt.Sprintf("http://localhost:%d", *port)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// A2A AgentCard discovery
	r.Get("/.well-known/agent-card.json", func(w http.ResponseWriter, _ *http.Request) {
		skills := make([]a2a.CardSkill, len(role.Skills))
		for i, s := range role.Skills {
			skills[i] = a2a.CardSkill{ID: s.ID, Name: s.Name, Description: s.Description}
		}

		card := a2a.AgentCard{
			Name:        role.Name,
			Description: role.Description,
			URL:         baseURL,
			Version:     "1.0.0",
			Capabilities: a2a.CardCapability{
				Streaming:         false,
				PushNotifications: false,
			},
			Skills:             skills,
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain"},
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
	log.Printf("%s listening on %s (model: %s)", role.Name, addr, client.client.Model())
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	var req a2a.JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a2a.WriteRPCError(w, "", -32700, "Parse error")
		return
	}

	if req.JSONRPC != "2.0" {
		a2a.WriteRPCError(w, req.ID, -32600, "Invalid request: jsonrpc must be 2.0")
		return
	}

	switch req.Method {
	case "message/send", "tasks/send":
		handleSendMessage(r.Context(), w, req)
	case "tasks/get":
		handleGetTask(w, req)
	case "tasks/cancel":
		handleCancelTask(w, req)
	default:
		a2a.WriteRPCError(w, req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func handleSendMessage(_ context.Context, w http.ResponseWriter, req a2a.JSONRPCRequest) {
	var params a2a.SendMessageParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		a2a.WriteRPCError(w, req.ID, -32602, "Invalid params")
		return
	}

	// Extract text from message parts
	var text string
	for _, part := range params.Message.Parts {
		if part.Text != "" {
			if text != "" {
				text += "\n"
			}
			text += part.Text
		}
	}

	if text == "" {
		a2a.WriteRPCError(w, req.ID, -32602, "Message must contain text")
		return
	}

	// Handle follow-up to existing task
	if params.TaskID != "" {
		if val, ok := tasks.Load(params.TaskID); ok {
			ts := val.(*taskState)
			ts.mu.Lock()

			if ts.Status == "input-required" {
				// Reset to working for follow-up processing
				ts.Status = "working"
				followContextID := ts.ContextID
				ts.mu.Unlock()

				// Process follow-up in background with history
				go func() {
					msgs := []chatMessage{
						{Role: "system", Content: role.SystemPrompt},
					}
					msgs = append(msgs, getHistory(followContextID)...)
					msgs = append(msgs, chatMessage{Role: "user", Content: text})

					tools := GetToolsForRole(role.ID)

					var response string
					var updatedHistory []chatMessage
					var err error

					if len(tools) > 0 {
						onToolCall := func(toolName, args string) {
							ts.mu.Lock()
							ts.ToolProgress = fmt.Sprintf("tool_call_started:%s:%s", toolName, args)
							ts.mu.Unlock()
						}
						onToolResult := func(toolName, summary string) {
							ts.mu.Lock()
							ts.ToolProgress = fmt.Sprintf("tool_call_completed:%s:%s", toolName, summary)
							ts.mu.Unlock()
						}
						response, updatedHistory, err = client.ChatWithTools(
							context.Background(), msgs, tools, onToolCall, onToolResult,
						)
					} else {
						response, err = client.ChatWithHistory(context.Background(), msgs)
					}

					ts.mu.Lock()
					defer ts.mu.Unlock()
					if err != nil {
						ts.Status = "failed"
						ts.Error = err.Error()
					} else {
						ts.Status = "completed"
						ts.Result = response
						ts.CompletedAt = time.Now()
						if updatedHistory != nil {
							conversations.Store(followContextID, updatedHistory)
						} else {
							appendHistory(followContextID,
								chatMessage{Role: "user", Content: text},
								chatMessage{Role: "assistant", Content: response},
							)
						}
					}
					scheduleCleanup(ts.ID)
				}()

				// Return immediately with "working" state
				a2a.WriteRPCResult(w, req.ID, a2a.A2ATask{
					ID:        ts.ID,
					ContextID: ts.ContextID,
					Status:    a2a.A2AStatus{State: "working"},
				})
				return
			}
			ts.mu.Unlock()
		}
	}

	// New task — create immediately, process in background
	taskID := uuid.New().String()
	contextID := params.ContextID
	if contextID == "" {
		contextID = uuid.New().String()
	}

	ts := &taskState{
		ID:        taskID,
		ContextID: contextID,
		Status:    "working",
		CreatedAt: time.Now(),
	}
	tasks.Store(taskID, ts)

	// Process in background goroutine with conversation history
	go func() {
		msgs := []chatMessage{
			{Role: "system", Content: role.SystemPrompt},
		}
		msgs = append(msgs, getHistory(contextID)...)
		msgs = append(msgs, chatMessage{Role: "user", Content: text})

		tools := GetToolsForRole(role.ID)

		var response string
		var updatedHistory []chatMessage
		var err error

		if len(tools) > 0 {
			onToolCall := func(toolName, args string) {
				ts.mu.Lock()
				ts.ToolProgress = fmt.Sprintf("tool_call_started:%s:%s", toolName, args)
				ts.mu.Unlock()
			}
			onToolResult := func(toolName, summary string) {
				ts.mu.Lock()
				ts.ToolProgress = fmt.Sprintf("tool_call_completed:%s:%s", toolName, summary)
				ts.mu.Unlock()
			}
			response, updatedHistory, err = client.ChatWithTools(
				context.Background(), msgs, tools, onToolCall, onToolResult,
			)
		} else {
			response, err = client.ChatWithHistory(context.Background(), msgs)
		}

		ts.mu.Lock()
		defer ts.mu.Unlock()
		if err != nil {
			ts.Status = "failed"
			ts.Error = err.Error()
			log.Printf("[%s] task %s failed: %v", role.Name, taskID, err)
		} else {
			ts.Status = "completed"
			ts.Result = response
			ts.CompletedAt = time.Now()
			// Store the full history including tool call/result messages
			if updatedHistory != nil {
				conversations.Store(contextID, updatedHistory)
			} else {
				appendHistory(contextID,
					chatMessage{Role: "user", Content: text},
					chatMessage{Role: "assistant", Content: response},
				)
			}
			log.Printf("[%s] task %s completed in %v", role.Name, taskID, ts.CompletedAt.Sub(ts.CreatedAt))
		}
		scheduleCleanup(taskID)
	}()

	// Return immediately with "working" state
	a2a.WriteRPCResult(w, req.ID, a2a.A2ATask{
		ID:        taskID,
		ContextID: contextID,
		Status:    a2a.A2AStatus{State: "working"},
	})
}

func handleGetTask(w http.ResponseWriter, req a2a.JSONRPCRequest) {
	var params a2a.TaskIDParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		a2a.WriteRPCError(w, req.ID, -32602, "Invalid params")
		return
	}

	val, ok := tasks.Load(params.ID)
	if !ok {
		a2a.WriteRPCError(w, req.ID, -32001, "Task not found")
		return
	}

	ts := val.(*taskState)
	ts.mu.Lock()
	defer ts.mu.Unlock()

	task := a2a.A2ATask{
		ID:        ts.ID,
		ContextID: ts.ContextID,
		Status:    a2a.A2AStatus{State: ts.Status},
	}

	switch ts.Status {
	case "working":
		if ts.ToolProgress != "" {
			task.Status.Message = &a2a.A2AMessage{
				Role:  "agent",
				Parts: []a2a.MessagePart{a2a.TextPart(ts.ToolProgress)},
			}
		}
	case "completed":
		if ts.Result != "" {
			task.Artifacts = []a2a.Artifact{{
				ArtifactID: uuid.New().String(),
				Parts:      []a2a.MessagePart{a2a.TextPart(ts.Result)},
			}}
		}
	case "failed":
		if ts.Error != "" {
			task.Status.Message = &a2a.A2AMessage{
				Role:  "agent",
				Parts: []a2a.MessagePart{a2a.TextPart(ts.Error)},
			}
		}
	case "input-required":
		task.Status.Message = &a2a.A2AMessage{
			Role:  "agent",
			Parts: []a2a.MessagePart{a2a.TextPart("Waiting for input")},
		}
	}

	a2a.WriteRPCResult(w, req.ID, task)
}

func handleCancelTask(w http.ResponseWriter, req a2a.JSONRPCRequest) {
	var params a2a.TaskIDParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		a2a.WriteRPCError(w, req.ID, -32602, "Invalid params")
		return
	}

	val, ok := tasks.Load(params.ID)
	if !ok {
		a2a.WriteRPCError(w, req.ID, -32001, "Task not found")
		return
	}

	ts := val.(*taskState)
	ts.mu.Lock()
	ts.Status = "canceled"
	ts.mu.Unlock()
	scheduleCleanup(ts.ID)

	a2a.WriteRPCResult(w, req.ID, a2a.A2ATask{
		ID:        ts.ID,
		ContextID: ts.ContextID,
		Status:    a2a.A2AStatus{State: "canceled"},
	})
}

// scheduleCleanup removes a task and its conversation history after a grace period.
func scheduleCleanup(taskID string) {
	time.AfterFunc(10*time.Minute, func() {
		if val, ok := tasks.Load(taskID); ok {
			ts := val.(*taskState)
			conversations.Delete(ts.ContextID)
		}
		tasks.Delete(taskID)
	})
}
