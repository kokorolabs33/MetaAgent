package models

import (
	"encoding/json"
	"time"
)

type Task struct {
	ID              string          `json:"id"`
	ConversationID  string          `json:"conversation_id,omitempty"`
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	Status          string          `json:"status"` // pending, planning, running, completed, failed, canceled
	CreatedBy       string          `json:"created_by"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
	Plan            json.RawMessage `json:"plan,omitempty"`
	Result          json.RawMessage `json:"result,omitempty"`
	Error           string          `json:"error,omitempty"`
	TemplateID      string          `json:"template_id,omitempty"`
	TemplateVersion int             `json:"template_version,omitempty"`
	ReplanCount     int             `json:"replan_count"`
	CreatedAt       time.Time       `json:"created_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`

	CompletedSubtasks int      `json:"completed_subtasks"`
	TotalSubtasks     int      `json:"total_subtasks"`
	AgentIDs          []string `json:"agent_ids"`
}

type SubTask struct {
	ID          string          `json:"id"`
	TaskID      string          `json:"task_id"`
	AgentID     string          `json:"agent_id"`
	Instruction string          `json:"instruction"`
	DependsOn   []string        `json:"depends_on"`
	Status      string          `json:"status"` // pending, running, completed, failed, input_required, canceled, blocked
	Input       json.RawMessage `json:"input,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	Error       string          `json:"error,omitempty"`
	A2ATaskID   string          `json:"a2a_task_id,omitempty"`
	Attempt     int             `json:"attempt"`
	MaxAttempts int             `json:"max_attempts"`
	CreatedAt   time.Time       `json:"created_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
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
