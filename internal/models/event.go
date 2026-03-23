package models

import (
	"encoding/json"
	"time"
)

type Event struct {
	ID        string          `json:"id"`
	TaskID    string          `json:"task_id"`
	SubtaskID string          `json:"subtask_id,omitempty"`
	Type      string          `json:"type"`
	ActorType string          `json:"actor_type"` // system, agent, user
	ActorID   string          `json:"actor_id,omitempty"`
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"created_at"`
}

type Message struct {
	ID         string          `json:"id"`
	TaskID     string          `json:"task_id"`
	SenderType string          `json:"sender_type"` // agent, user, system
	SenderID   string          `json:"sender_id,omitempty"`
	SenderName string          `json:"sender_name"`
	Content    string          `json:"content"`
	Mentions   []string        `json:"mentions"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}
