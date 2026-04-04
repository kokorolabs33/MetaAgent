package models

import (
	"encoding/json"
	"time"
)

// Agent represents a registered A2A agent.
type Agent struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Version         string          `json:"version"`
	Description     string          `json:"description"`
	Endpoint        string          `json:"endpoint"`
	AgentCardURL    string          `json:"agent_card_url"`
	AgentCard       json.RawMessage `json:"agent_card,omitempty"`
	CardFetchedAt   *time.Time      `json:"card_fetched_at,omitempty"`
	Capabilities    []string        `json:"capabilities"`
	Skills          json.RawMessage `json:"skills,omitempty"`
	Status          string          `json:"status"`
	IsOnline        bool            `json:"is_online"`
	LastHealthCheck *time.Time      `json:"last_health_check,omitempty"`
	SkillHash       string          `json:"skill_hash,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}
