package main

import (
	"context"
	"fmt"
	"os"

	"taskhub/internal/llm"
)

// OpenAIClient wraps the shared llm.Client for the openaiagent binary.
// Unlike the server, the agent binary requires the API key to be set.
type OpenAIClient struct {
	client *llm.Client
}

// NewOpenAIClient creates a client from environment variables.
// Requires OPENAI_API_KEY; exits if not set.
func NewOpenAIClient() *OpenAIClient {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENAI_API_KEY environment variable is required")
		os.Exit(1)
	}

	return &OpenAIClient{
		client: llm.NewClient(apiKey),
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatWithHistory sends a request with full conversation history.
func (c *OpenAIClient) ChatWithHistory(ctx context.Context, messages []chatMessage) (string, error) {
	llmMsgs := make([]llm.ChatMessage, len(messages))
	for i, m := range messages {
		llmMsgs[i] = llm.ChatMessage{Role: m.Role, Content: m.Content}
	}
	return c.client.ChatWithHistory(ctx, llmMsgs)
}

// Chat sends a single-shot request with a system prompt and user message,
// returning the assistant's response text.
func (c *OpenAIClient) Chat(ctx context.Context, systemPrompt string, userMessage string) (string, error) {
	return c.client.Chat(ctx, systemPrompt, userMessage)
}
