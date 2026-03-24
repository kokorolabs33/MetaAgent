package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ClaudeClient calls the Claude CLI for LLM inference.
type ClaudeClient struct{}

// NewClaudeClient creates a new CLI-based client.
func NewClaudeClient() *ClaudeClient {
	return &ClaudeClient{}
}

// Message is a conversation message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Chat calls Claude CLI with the system prompt and conversation.
// For multi-turn, it concatenates the conversation into the user message
// since claude --print is single-turn.
func (c *ClaudeClient) Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error) {
	// Build user message from conversation history
	var userMsg string
	if len(messages) == 1 {
		userMsg = messages[0].Content
	} else {
		// Multi-turn: format conversation as context
		var sb strings.Builder
		for _, m := range messages {
			if m.Role == "user" {
				sb.WriteString("User: ")
			} else {
				sb.WriteString("Assistant: ")
			}
			sb.WriteString(m.Content)
			sb.WriteString("\n\n")
		}
		userMsg = strings.TrimSpace(sb.String())
	}

	cmd := exec.CommandContext(ctx, "claude", "--print", "--system-prompt", systemPrompt, "--output-format", "text", userMsg)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude cli exited %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("claude cli: %w", err)
	}

	result := strings.TrimSpace(string(out))
	return result, nil
}
