package models

import (
	"encoding/json"
	"time"
)

type Agent struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	SystemPrompt string    `json:"system_prompt"`
	Capabilities []string  `json:"capabilities"`
	Color        string    `json:"color"`
	CreatedAt    time.Time `json:"created_at"`
}

type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type Channel struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Document  string    `json:"document"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type Message struct {
	ID         string    `json:"id"`
	ChannelID  string    `json:"channel_id"`
	SenderID   string    `json:"sender_id"`
	SenderName string    `json:"sender_name"`
	Content    string    `json:"content"`
	Type       string    `json:"type"`
	CreatedAt  time.Time `json:"created_at"`
}

type ChannelAgent struct {
	ChannelID string `json:"channel_id"`
	AgentID   string `json:"agent_id"`
	Status    string `json:"status"`
}

// CapabilitiesToJSON serializes []string to JSON string for DB storage.
func CapabilitiesToJSON(caps []string) string {
	b, _ := json.Marshal(caps)
	return string(b)
}

// JSONToCapabilities deserializes JSON string from DB to []string.
func JSONToCapabilities(s string) []string {
	var caps []string
	_ = json.Unmarshal([]byte(s), &caps)
	return caps
}
