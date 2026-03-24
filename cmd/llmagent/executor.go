package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
)

// LLMExecutor handles A2A messages by calling the Claude API.
type LLMExecutor struct {
	role   Role
	claude *ClaudeClient

	// Conversation history per context (contextId -> messages).
	mu     sync.Mutex
	convos map[string][]Message
}

// NewLLMExecutor creates an executor for the given role.
func NewLLMExecutor(role Role, claude *ClaudeClient) *LLMExecutor {
	return &LLMExecutor{
		role:   role,
		claude: claude,
		convos: make(map[string][]Message),
	}
}

// HandleMessage processes an incoming A2A message and returns the LLM response.
// For the finance role, it also detects needs_input in the response JSON.
func (e *LLMExecutor) HandleMessage(
	ctx context.Context,
	contextID string,
	text string,
	data map[string]any,
) (responseText string, inputRequired bool, inputMessage string, err error) {
	// Build user message content
	userContent := text
	if data != nil {
		dataJSON, jsonErr := json.MarshalIndent(data, "", "  ")
		if jsonErr != nil {
			return "", false, "", fmt.Errorf("marshal data: %w", jsonErr)
		}
		userContent = text + "\n\nUpstream analysis data:\n" + string(dataJSON)
	}

	e.mu.Lock()
	convo := e.convos[contextID]
	convo = append(convo, Message{Role: "user", Content: userContent})
	e.convos[contextID] = convo
	// Copy for use outside the lock.
	msgs := make([]Message, len(convo))
	copy(msgs, convo)
	e.mu.Unlock()

	// Call Claude
	response, err := e.claude.Chat(ctx, e.role.SystemPrompt, msgs)
	if err != nil {
		return "", false, "", fmt.Errorf("claude call: %w", err)
	}

	// Store assistant response in conversation history
	e.mu.Lock()
	e.convos[contextID] = append(e.convos[contextID], Message{Role: "assistant", Content: response})
	e.mu.Unlock()

	// For finance role: check if LLM output contains needs_input
	if e.role.ID == "finance" {
		cleaned := stripCodeFences(response)
		var parsed map[string]any
		if jsonErr := json.Unmarshal([]byte(cleaned), &parsed); jsonErr == nil {
			if ni, ok := parsed["needs_input"].(map[string]any); ok {
				msg, _ := ni["message"].(string)
				if msg != "" {
					log.Printf("finance agent: needs_input triggered: %s", msg)
					return response, true, msg, nil
				}
			}
		}
	}

	return response, false, "", nil
}

// HandleFollowUp processes a follow-up message for an existing conversation
// (e.g. after an input-required state).
func (e *LLMExecutor) HandleFollowUp(ctx context.Context, contextID string, text string) (string, error) {
	e.mu.Lock()
	convo := e.convos[contextID]
	convo = append(convo, Message{Role: "user", Content: text})
	e.convos[contextID] = convo
	msgs := make([]Message, len(convo))
	copy(msgs, convo)
	e.mu.Unlock()

	response, err := e.claude.Chat(ctx, e.role.SystemPrompt, msgs)
	if err != nil {
		return "", fmt.Errorf("claude follow-up: %w", err)
	}

	e.mu.Lock()
	e.convos[contextID] = append(e.convos[contextID], Message{Role: "assistant", Content: response})
	e.mu.Unlock()

	return response, nil
}

// stripCodeFences removes markdown code fences that LLMs sometimes wrap around JSON.
func stripCodeFences(s string) string {
	cleaned := strings.TrimSpace(s)
	if strings.HasPrefix(cleaned, "```") {
		lines := strings.Split(cleaned, "\n")
		if len(lines) > 2 {
			cleaned = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	return strings.TrimSpace(cleaned)
}
