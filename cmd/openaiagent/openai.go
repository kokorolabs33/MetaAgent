package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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

// chatMessage extends the base message struct with OpenAI tool calling fields.
// Per D-09: only extend the openaiagent-local struct, not internal/llm.ChatMessage.
type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// toolCall represents a tool invocation returned by the model.
type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function toolCallFunc `json:"function"`
}

// toolCallFunc describes the function name and arguments for a tool call.
type toolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
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

// ToolCallHook is invoked when the model requests a tool call.
type ToolCallHook func(toolName, args string)

// ToolResultHook is invoked when a tool call completes.
type ToolResultHook func(toolName, summary string)

// toolChatRequest is the request body for OpenAI Chat Completions with tools.
type toolChatRequest struct {
	Model             string        `json:"model"`
	Messages          []chatMessage `json:"messages"`
	Tools             []toolDef     `json:"tools,omitempty"`
	ParallelToolCalls *bool         `json:"parallel_tool_calls,omitempty"`
}

// toolDef describes a tool for the OpenAI API.
type toolDef struct {
	Type     string      `json:"type"`
	Function toolFuncDef `json:"function"`
}

// toolFuncDef describes a function tool definition.
type toolFuncDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	Strict      bool            `json:"strict,omitempty"`
}

// toolChatResponse is the response from OpenAI Chat Completions with tools.
type toolChatResponse struct {
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []toolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// maxToolRounds is the safety limit for tool call iterations.
// Per T-07-03: prevents infinite loops from adversarial tool call patterns.
const maxToolRounds = 5

// ChatWithTools implements the full tool calling loop:
//  1. Send request with tools array
//  2. If finish_reason == "tool_calls", execute each tool sequentially
//  3. Append assistant message (with tool_calls) and tool result messages
//  4. Send updated history back to the API
//  5. Repeat until finish_reason == "stop" or maxToolRounds reached
//
// Returns the final assistant text, the full updated conversation history, and any error.
// Per Pitfall 1: every assistant message with tool_calls is followed by exactly one
// tool result message per call ID. Per Pitfall 10: tools execute sequentially.
func (c *OpenAIClient) ChatWithTools(
	ctx context.Context,
	messages []chatMessage,
	tools []ToolDefinition,
	onToolCall ToolCallHook,
	onToolResult ToolResultHook,
) (string, []chatMessage, error) {
	// Build OpenAI tool definitions from our ToolDefinition structs.
	apiTools := make([]toolDef, len(tools))
	for i, t := range tools {
		apiTools[i] = toolDef{
			Type: "function",
			Function: toolFuncDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
				Strict:      true,
			},
		}
	}

	// Build tool lookup for execution.
	toolMap := make(map[string]ToolDefinition, len(tools))
	for _, t := range tools {
		toolMap[t.Name] = t
	}

	// Per Pitfall 10: disable parallel tool calls.
	parallelCalls := false

	history := make([]chatMessage, len(messages))
	copy(history, messages)

	for round := 0; round < maxToolRounds; round++ {
		reqBody := toolChatRequest{
			Model:             c.client.Model(),
			Messages:          history,
			Tools:             apiTools,
			ParallelToolCalls: &parallelCalls,
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			return "", nil, fmt.Errorf("chatWithTools: marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
		if err != nil {
			return "", nil, fmt.Errorf("chatWithTools: create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.client.APIKey())

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", nil, fmt.Errorf("chatWithTools: send request: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", nil, fmt.Errorf("chatWithTools: read response: %w", err)
		}

		var chatResp toolChatResponse
		if err := json.Unmarshal(respBody, &chatResp); err != nil {
			return "", nil, fmt.Errorf("chatWithTools: unmarshal response: %w", err)
		}

		if chatResp.Error != nil {
			return "", nil, fmt.Errorf("chatWithTools: openai error: %s", chatResp.Error.Message)
		}

		if len(chatResp.Choices) == 0 {
			return "", nil, fmt.Errorf("chatWithTools: no response choices")
		}

		choice := chatResp.Choices[0]

		// If finish_reason is "stop" (or anything other than "tool_calls"), return the final text.
		if choice.FinishReason != "tool_calls" {
			// Append the final assistant message to history.
			history = append(history, chatMessage{
				Role:    "assistant",
				Content: choice.Message.Content,
			})
			return choice.Message.Content, history, nil
		}

		// Model wants to call tools. Append assistant message with tool_calls (no content).
		assistantMsg := chatMessage{
			Role:      "assistant",
			ToolCalls: choice.Message.ToolCalls,
		}
		history = append(history, assistantMsg)

		// Execute each tool call sequentially (per Pitfall 10).
		for _, tc := range choice.Message.ToolCalls {
			if onToolCall != nil {
				onToolCall(tc.Function.Name, tc.Function.Arguments)
			}

			tool, ok := toolMap[tc.Function.Name]
			var result string
			if !ok {
				result = fmt.Sprintf("Error: unknown tool '%s'", tc.Function.Name)
				log.Printf("chatWithTools: unknown tool %q requested by model", tc.Function.Name)
			} else {
				var execErr error
				result, execErr = tool.Execute(ctx, json.RawMessage(tc.Function.Arguments))
				if execErr != nil {
					result = fmt.Sprintf("Tool error: %v", execErr)
					log.Printf("chatWithTools: tool %q execution failed: %v", tc.Function.Name, execErr)
				}
			}

			if onToolResult != nil {
				// Provide a short summary (first 100 chars).
				summary := result
				if len(summary) > 100 {
					summary = summary[:100] + "..."
				}
				onToolResult(tc.Function.Name, summary)
			}

			// Append tool result message per Pitfall 1: must follow assistant tool_calls
			// with exactly one tool result per call ID.
			history = append(history, chatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
			})
		}
	}

	// Exceeded max rounds -- return whatever we have.
	log.Printf("chatWithTools: reached max tool call rounds (%d)", maxToolRounds)
	return "I was unable to complete the task within the allowed number of tool call iterations.", history, nil
}
