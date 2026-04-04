package models

import (
	"encoding/json"
	"time"
)

type WorkflowTemplate struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Version      int             `json:"version"`
	Steps        json.RawMessage `json:"steps"`
	Variables    json.RawMessage `json:"variables"`
	SourceTaskID string          `json:"source_task_id,omitempty"`
	IsActive     bool            `json:"is_active"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type TemplateVersion struct {
	ID                string          `json:"id"`
	TemplateID        string          `json:"template_id"`
	Version           int             `json:"version"`
	Steps             json.RawMessage `json:"steps"`
	Source            string          `json:"source"`
	Changes           json.RawMessage `json:"changes"`
	BasedOnExecutions int             `json:"based_on_executions"`
	CreatedAt         time.Time       `json:"created_at"`
}

type TemplateExecution struct {
	ID                string          `json:"id"`
	TemplateID        string          `json:"template_id"`
	TemplateVersion   int             `json:"template_version"`
	TaskID            string          `json:"task_id"`
	ActualSteps       json.RawMessage `json:"actual_steps"`
	HITLInterventions int             `json:"hitl_interventions"`
	ReplanCount       int             `json:"replan_count"`
	Outcome           string          `json:"outcome"`
	DurationSeconds   int             `json:"duration_seconds"`
	CreatedAt         time.Time       `json:"created_at"`
}
