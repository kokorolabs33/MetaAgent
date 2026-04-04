package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Server is a JSON-RPC 2.0 handler that exposes TaskHub as an A2A agent.
type Server struct {
	DB           *pgxpool.Pool
	Aggregator   *Aggregator
	BaseURL      string
	TaskExecutor func(ctx context.Context, taskID string) error
}

// a2aServerTask is the task object returned in server-side JSON-RPC results.
// It uses a distinct name from the client-side a2aTask to avoid redeclaring
// the type while allowing a slightly different shape (no contextId).
type a2aServerTask struct {
	ID        string     `json:"id"`
	Status    a2aStatus  `json:"status"`
	Artifacts []artifact `json:"artifacts,omitempty"`
}

// HandleJSONRPC is the main HTTP handler for POST /a2a.
// It parses the JSON-RPC request, dispatches to the appropriate method handler,
// and writes a JSON-RPC response.
func (s *Server) HandleJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeRPCError(w, "", -32600, "only POST is supported")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeRPCError(w, "", -32700, "could not read request body")
		return
	}

	var req jsonRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeRPCError(w, "", -32700, "invalid JSON")
		return
	}

	if req.JSONRPC != "2.0" {
		writeRPCError(w, req.ID, -32600, "jsonrpc field must be \"2.0\"")
		return
	}

	// Re-marshal params so method handlers can unmarshal into their specific structs.
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		writeRPCError(w, req.ID, -32602, "invalid params")
		return
	}

	switch req.Method {
	case "tasks/send", "message/send":
		result, rpcErr := s.handleTasksSend(r.Context(), paramsJSON)
		if rpcErr != nil {
			writeRPCError(w, req.ID, rpcErr.Code, rpcErr.Message)
			return
		}
		writeRPCResult(w, req.ID, result)

	case "tasks/get":
		result, rpcErr := s.handleTasksGet(r.Context(), paramsJSON)
		if rpcErr != nil {
			writeRPCError(w, req.ID, rpcErr.Code, rpcErr.Message)
			return
		}
		writeRPCResult(w, req.ID, result)

	case "tasks/cancel":
		result, rpcErr := s.handleTasksCancel(r.Context(), paramsJSON)
		if rpcErr != nil {
			writeRPCError(w, req.ID, rpcErr.Code, rpcErr.Message)
			return
		}
		writeRPCResult(w, req.ID, result)

	default:
		writeRPCError(w, req.ID, -32601, fmt.Sprintf("method %q not found", req.Method))
	}
}

// handleTasksSend creates an internal task from an incoming A2A message,
// spawns the task executor in a background goroutine, and returns immediately
// with state="working".
func (s *Server) handleTasksSend(ctx context.Context, paramsJSON []byte) (*a2aServerTask, *jsonRPCError) {
	var sp sendMessageParams
	if err := json.Unmarshal(paramsJSON, &sp); err != nil {
		return nil, &jsonRPCError{Code: -32602, Message: "invalid params for tasks/send"}
	}

	// Extract text instruction from message parts.
	var instruction string
	for _, part := range sp.Message.Parts {
		if part.Text != "" {
			instruction = part.Text
			break
		}
	}
	if instruction == "" {
		return nil, &jsonRPCError{Code: -32602, Message: "message must contain at least one text part"}
	}

	taskID := uuid.New().String()
	callerTaskID := sp.TaskID
	now := time.Now().UTC()

	_, err := s.DB.Exec(ctx,
		`INSERT INTO tasks (id, title, description, status, created_by, source, caller_task_id, replan_count, created_at)
		 VALUES ($1, $2, $3, 'pending', 'a2a', 'a2a', $4, 0, $5)`,
		taskID, instruction, instruction, callerTaskID, now)
	if err != nil {
		log.Printf("a2a server: create task: %v", err)
		return nil, &jsonRPCError{Code: -32000, Message: "internal error creating task"}
	}

	// Spawn the task executor in a background goroutine.
	if s.TaskExecutor != nil {
		go func() {
			if err := s.TaskExecutor(context.Background(), taskID); err != nil {
				log.Printf("a2a server: execute task %s: %v", taskID, err)
			}
		}()
	}

	return &a2aServerTask{
		ID: taskID,
		Status: a2aStatus{
			State: "working",
		},
	}, nil
}

// handleTasksGet queries an internal task and maps its status to A2A state.
func (s *Server) handleTasksGet(ctx context.Context, paramsJSON []byte) (*a2aServerTask, *jsonRPCError) {
	var tp taskIDParams
	if err := json.Unmarshal(paramsJSON, &tp); err != nil {
		return nil, &jsonRPCError{Code: -32602, Message: "invalid params for tasks/get"}
	}

	if tp.ID == "" {
		return nil, &jsonRPCError{Code: -32602, Message: "task id is required"}
	}

	var status string
	var result []byte
	var taskError *string

	err := s.DB.QueryRow(ctx,
		`SELECT status, result, error FROM tasks WHERE id = $1`, tp.ID,
	).Scan(&status, &result, &taskError)
	if err != nil {
		return nil, &jsonRPCError{Code: -32001, Message: "task not found"}
	}

	task := &a2aServerTask{
		ID: tp.ID,
		Status: a2aStatus{
			State: mapStatusToA2AState(status),
		},
	}

	// If the task failed, include the error message.
	if taskError != nil && *taskError != "" {
		task.Status.Message = &a2aMessage{
			Role:  "agent",
			Parts: []MessagePart{TextPart(*taskError)},
		}
	}

	// If the task completed with a result, include it as an artifact.
	if status == "completed" && result != nil {
		task.Artifacts = []artifact{
			{
				ArtifactID: "result",
				Parts:      []MessagePart{DataPart(json.RawMessage(result))},
			},
		}
	}

	return task, nil
}

// handleTasksCancel cancels an internal task if it is not already in a terminal state.
func (s *Server) handleTasksCancel(ctx context.Context, paramsJSON []byte) (*a2aServerTask, *jsonRPCError) {
	var tp taskIDParams
	if err := json.Unmarshal(paramsJSON, &tp); err != nil {
		return nil, &jsonRPCError{Code: -32602, Message: "invalid params for tasks/cancel"}
	}

	if tp.ID == "" {
		return nil, &jsonRPCError{Code: -32602, Message: "task id is required"}
	}

	tag, err := s.DB.Exec(ctx,
		`UPDATE tasks SET status = 'canceled'
		 WHERE id = $1 AND status NOT IN ('completed', 'failed', 'canceled')`,
		tp.ID)
	if err != nil {
		return nil, &jsonRPCError{Code: -32000, Message: "internal error canceling task"}
	}

	if tag.RowsAffected() == 0 {
		// Either not found or already terminal — check which.
		var exists bool
		_ = s.DB.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM tasks WHERE id = $1)`, tp.ID,
		).Scan(&exists)

		if !exists {
			return nil, &jsonRPCError{Code: -32001, Message: "task not found"}
		}
		// Task exists but is already in a terminal state; return current state.
	}

	return &a2aServerTask{
		ID: tp.ID,
		Status: a2aStatus{
			State: "canceled",
		},
	}, nil
}

// mapStatusToA2AState converts an internal task status to an A2A protocol state.
func mapStatusToA2AState(status string) string {
	switch status {
	case "pending", "planning":
		return "submitted"
	case "running":
		return "working"
	case "completed":
		return "completed"
	case "failed":
		return "failed"
	case "canceled":
		return "canceled"
	default:
		return "unknown"
	}
}

// writeRPCResult writes a successful JSON-RPC 2.0 response.
func writeRPCResult(w http.ResponseWriter, id string, result any) {
	data, err := json.Marshal(result)
	if err != nil {
		writeRPCError(w, id, -32000, "internal error marshaling result")
		return
	}

	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  data,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("a2a server: write response: %v", err)
	}
}

// writeRPCError writes an error JSON-RPC 2.0 response.
func writeRPCError(w http.ResponseWriter, id string, code int, message string) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &jsonRPCError{Code: code, Message: message},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("a2a server: write error response: %v", err)
	}
}
