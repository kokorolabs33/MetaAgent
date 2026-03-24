package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const claudeAPIURL = "https://api.anthropic.com/v1/messages"

// ClaudeClient calls the Anthropic Messages API.
type ClaudeClient struct {
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
	HTTPClient  *http.Client
}

// NewClaudeClient creates a client with defaults from environment variables.
func NewClaudeClient() *ClaudeClient {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	model := os.Getenv("CLAUDE_MODEL")
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	return &ClaudeClient{
		APIKey:      apiKey,
		Model:       model,
		MaxTokens:   4096,
		Temperature: 0.3,
		HTTPClient: &http.Client{
			Timeout: 3 * time.Minute,
		},
	}
}

// Message is a conversation message for the Claude API.
type Message struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// claudeRequest is the Messages API request body.
type claudeRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature,omitempty"`
	System      string    `json:"system,omitempty"`
	Messages    []Message `json:"messages"`
}

// claudeResponse is the Messages API response body.
type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Chat sends a conversation to Claude and returns the assistant's text response.
func (c *ClaudeClient) Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error) {
	reqBody := claudeRequest{
		Model:       c.Model,
		MaxTokens:   c.MaxTokens,
		Temperature: c.Temperature,
		System:      systemPrompt,
		Messages:    messages,
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeAPIURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("api call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claude API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	for _, block := range claudeResp.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("claude response contained no text block")
}
