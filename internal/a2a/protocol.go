// Protocol types shared between A2A client, server, and agent implementations.
// Agent binaries (e.g. cmd/openaiagent) import these to avoid duplicating
// the JSON-RPC and A2A wire-format structs.

package a2a

import (
	"encoding/json"
	"log"
	"net/http"
)

// JSONRPCRequest is the JSON-RPC 2.0 request envelope.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	ID      string          `json:"id"`
	Params  json.RawMessage `json:"params"`
}

// JSONRPCResponse is the JSON-RPC 2.0 response envelope.
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      string        `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError is the error object in a JSON-RPC 2.0 response.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// A2ATask is the A2A protocol task object.
type A2ATask struct {
	ID        string     `json:"id"`
	ContextID string     `json:"contextId,omitempty"`
	Status    A2AStatus  `json:"status"`
	Artifacts []Artifact `json:"artifacts,omitempty"`
}

// A2AStatus represents the status of an A2A task.
type A2AStatus struct {
	State   string      `json:"state"`
	Message *A2AMessage `json:"message,omitempty"`
}

// A2AMessage is an A2A protocol message with role and parts.
type A2AMessage struct {
	Role  string        `json:"role"`
	Parts []MessagePart `json:"parts"`
}

// Artifact is an output artifact from a task.
type Artifact struct {
	ArtifactID string        `json:"artifactId,omitempty"`
	Parts      []MessagePart `json:"parts,omitempty"`
}

// SendMessageParams is the params for tasks/send and message/send methods.
type SendMessageParams struct {
	Message   A2AMessage `json:"message"`
	ContextID string     `json:"contextId,omitempty"`
	TaskID    string     `json:"taskId,omitempty"`
}

// TaskIDParams is the params for tasks/get and tasks/cancel methods.
type TaskIDParams struct {
	ID string `json:"id"`
}

// WriteRPCResult writes a successful JSON-RPC 2.0 response.
func WriteRPCResult(w http.ResponseWriter, id string, result any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}); err != nil {
		log.Printf("a2a: write RPC result: %v", err)
	}
}

// WriteRPCError writes an error JSON-RPC 2.0 response.
func WriteRPCError(w http.ResponseWriter, id string, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &JSONRPCError{Code: code, Message: message},
	}); err != nil {
		log.Printf("a2a: write RPC error: %v", err)
	}
}
