package models

import (
	"encoding/json"
	"time"
)

type Task struct {
	ID          string          `json:"id"`
	OrgID       string          `json:"org_id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Status      string          `json:"status"` // pending, planning, running, completed, failed, cancelled
	CreatedBy   string          `json:"created_by"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	Plan        json.RawMessage `json:"plan,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
	ReplanCount int             `json:"replan_count"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

type SubTask struct {
	ID           string          `json:"id"`
	TaskID       string          `json:"task_id"`
	AgentID      string          `json:"agent_id"`
	Instruction  string          `json:"instruction"`
	DependsOn    []string        `json:"depends_on"`
	Status       string          `json:"status"` // pending, running, completed, failed, waiting_for_input, cancelled, blocked
	Input        json.RawMessage `json:"input,omitempty"`
	Output       json.RawMessage `json:"output,omitempty"`
	Error        string          `json:"error,omitempty"`
	PollJobID    string          `json:"poll_job_id,omitempty"`
	PollEndpoint string          `json:"poll_endpoint,omitempty"`
	Attempt      int             `json:"attempt"`
	MaxAttempts  int             `json:"max_attempts"`
	CreatedAt    time.Time       `json:"created_at"`
	StartedAt    *time.Time      `json:"started_at,omitempty"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
}

// TaskWithSubtasks is the combined view for task detail API.
type TaskWithSubtasks struct {
	Task
	SubTasks []SubTask `json:"subtasks"`
}

// ExecutionPlan is what the orchestrator returns.
type ExecutionPlan struct {
	Summary  string        `json:"summary"`
	SubTasks []PlanSubTask `json:"subtasks"`
}

type PlanSubTask struct {
	ID          string   `json:"id"` // temp ID for dependency references
	AgentID     string   `json:"agent_id"`
	AgentName   string   `json:"agent_name"`
	Instruction string   `json:"instruction"`
	DependsOn   []string `json:"depends_on"` // references to other PlanSubTask IDs
}
