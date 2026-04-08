package models

import "time"

// Conversation is the top-level entity in the chat-first UI.
// Messages and tasks belong to a conversation.
type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ConversationListItem is the sidebar view of a conversation.
type ConversationListItem struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	AgentCount   int    `json:"agent_count"`
	TaskCount    int    `json:"task_count"`
	LatestStatus string `json:"latest_status"` // status of most recent task
	UpdatedAt    string `json:"updated_at"`
}
