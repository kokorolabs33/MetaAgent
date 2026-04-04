package models

import (
	"encoding/json"
	"time"
)

type Policy struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Rules     json.RawMessage `json:"rules"`
	Priority  int             `json:"priority"`
	IsActive  bool            `json:"is_active"`
	CreatedAt time.Time       `json:"created_at"`
}

type A2AServerConfig struct {
	ID                  int             `json:"id"`
	Enabled             bool            `json:"enabled"`
	NameOverride        *string         `json:"name_override,omitempty"`
	DescriptionOverride *string         `json:"description_override,omitempty"`
	SecurityScheme      json.RawMessage `json:"security_scheme"`
	AggregatedCard      json.RawMessage `json:"aggregated_card"`
	CardUpdatedAt       *time.Time      `json:"card_updated_at,omitempty"`
}
