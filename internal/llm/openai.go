// Package llm provides a shared OpenAI Chat Completions API client.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// Client calls the OpenAI Chat Completions API via direct HTTP requests.
type Client struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// ChatMessage represents a single message in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// NewClient creates a new OpenAI client with the given API key.
// If apiKey is empty, a warning is logged but the client is still returned;
// calls will fail at request time.
func NewClient(apiKey string) *Client {
	if apiKey == "" {
		log.Println("llm: OPENAI_API_KEY not set -- task orchestration will fail")
	}

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	return &Client{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{},
	}
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Model returns the model name the client is configured to use.
func (c *Client) Model() string {
	return c.model
}

// Chat sends a single-shot request with a system prompt and user message,
// returning the assistant's response text.
func (c *Client) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	return c.ChatWithHistory(ctx, []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	})
}

// ChatWithHistory sends a request with full conversation history.
func (c *Client) ChatWithHistory(ctx context.Context, messages []ChatMessage) (string, error) {
	reqBody := chatRequest{
		Model:    c.model,
		Messages: messages,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("llm chat: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("llm chat: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm chat: send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("llm chat: read response: %w", err)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("llm chat: unmarshal response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("llm chat: openai error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("llm chat: no response choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}
