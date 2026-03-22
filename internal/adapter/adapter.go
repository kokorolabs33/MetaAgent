// internal/adapter/adapter.go
package adapter

import (
	"context"
	"encoding/json"

	"taskhub/internal/models"
)

// JobHandle is returned by Submit and used for Poll/SendInput.
type JobHandle struct {
	JobID          string `json:"job_id"`
	StatusEndpoint string `json:"status_endpoint,omitempty"`
}

// AgentStatus is the normalized response from Poll.
type AgentStatus struct {
	Status       string          `json:"status"` // "running" | "completed" | "failed" | "needs_input"
	Progress     *float64        `json:"progress,omitempty"`
	Result       json.RawMessage `json:"result,omitempty"`
	Error        string          `json:"error,omitempty"`
	InputRequest *InputRequest   `json:"input_request,omitempty"`
	Messages     []AgentMessage  `json:"messages,omitempty"`
}

// InputRequest is sent by an agent when it needs user input.
type InputRequest struct {
	Message string   `json:"message"`
	Options []string `json:"options,omitempty"`
}

// AgentMessage is a message emitted by an agent during execution.
type AgentMessage struct {
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}

// SubTaskInput is what the platform sends to an agent.
type SubTaskInput struct {
	TaskID      string          `json:"task_id"`
	Instruction string          `json:"instruction"`
	Input       json.RawMessage `json:"input,omitempty"`
	CallbackURL string          `json:"callback_url,omitempty"`
}

// UserInput is what the user sends back when an agent requests input.
type UserInput struct {
	SubtaskID string          `json:"subtask_id"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// AgentAdapter is the interface all adapters implement.
type AgentAdapter interface {
	Submit(ctx context.Context, agent models.Agent, input SubTaskInput) (JobHandle, error)
	Poll(ctx context.Context, agent models.Agent, handle JobHandle) (AgentStatus, error)
	SendInput(ctx context.Context, agent models.Agent, handle JobHandle, input UserInput) error
}
