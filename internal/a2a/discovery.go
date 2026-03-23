package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AgentCard is the A2A agent card returned by /.well-known/agent-card.json.
type AgentCard struct {
	Name               string         `json:"name"`
	Description        string         `json:"description"`
	URL                string         `json:"url"`
	Version            string         `json:"version"`
	Capabilities       CardCapability `json:"capabilities,omitempty"`
	Skills             []CardSkill    `json:"skills,omitempty"`
	DefaultInputModes  []string       `json:"defaultInputModes,omitempty"`
	DefaultOutputModes []string       `json:"defaultOutputModes,omitempty"`
}

// CardCapability describes what the agent supports.
type CardCapability struct {
	Streaming              bool `json:"streaming,omitempty"`
	PushNotifications      bool `json:"pushNotifications,omitempty"`
	StateTransitionHistory bool `json:"stateTransitionHistory,omitempty"`
}

// CardSkill is a skill advertised by the agent.
type CardSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// DiscoveredAgent holds the parsed AgentCard data for registration.
type DiscoveredAgent struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Version      string          `json:"version"`
	Skills       []CardSkill     `json:"skills"`
	Capabilities CardCapability  `json:"capabilities"`
	URL          string          `json:"url"`
	RawCard      json.RawMessage `json:"raw_card"`
}

// Discover fetches and validates an AgentCard from the given base URL.
func Discover(ctx context.Context, baseURL string) (*DiscoveredAgent, error) {
	cardURL := strings.TrimRight(baseURL, "/") + "/.well-known/agent-card.json"

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cardURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch agent card from %s: %w", cardURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent card returned HTTP %d from %s", resp.StatusCode, cardURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read agent card response: %w", err)
	}

	var card AgentCard
	if err := json.Unmarshal(body, &card); err != nil {
		return nil, fmt.Errorf("invalid agent card JSON: %w", err)
	}

	// Validate required fields
	if card.Name == "" {
		return nil, fmt.Errorf("agent card missing required field: name")
	}

	// If the card doesn't specify a URL, use the base URL
	if card.URL == "" {
		card.URL = baseURL
	}

	skills := card.Skills
	if skills == nil {
		skills = []CardSkill{}
	}

	return &DiscoveredAgent{
		Name:         card.Name,
		Description:  card.Description,
		Version:      card.Version,
		Skills:       skills,
		Capabilities: card.Capabilities,
		URL:          card.URL,
		RawCard:      json.RawMessage(body),
	}, nil
}
