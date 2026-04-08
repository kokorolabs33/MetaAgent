package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// streamChunk represents a single SSE chunk from the OpenAI streaming API.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content,omitempty"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id,omitempty"`
				Type     string `json:"type,omitempty"`
				Function struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				} `json:"function"`
			} `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// toolCallAccumulator accumulates streamed tool call deltas keyed by index.
type toolCallAccumulator struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

// streamChatRequest extends toolChatRequest with streaming fields.
type streamChatRequest struct {
	Model             string        `json:"model"`
	Messages          []chatMessage `json:"messages"`
	Tools             []toolDef     `json:"tools,omitempty"`
	ParallelToolCalls *bool         `json:"parallel_tool_calls,omitempty"`
	Stream            bool          `json:"stream"`
	StreamOptions     *streamOpts   `json:"stream_options,omitempty"`
}

type streamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

// deltaHTTPClient is a shared HTTP client for sending deltas with a short timeout.
var deltaHTTPClient = &http.Client{Timeout: 2 * time.Second}

// sendDelta POSTs a streaming delta to the platform server.
// Errors are logged but do not fail the stream (dropped deltas are acceptable).
func sendDelta(ctx context.Context, platformURL, taskID, subtaskID, agentID, deltaText string, done bool) error {
	payload, err := json.Marshal(map[string]any{
		"task_id":    taskID,
		"subtask_id": subtaskID,
		"agent_id":   agentID,
		"delta_text": deltaText,
		"done":       done,
	})
	if err != nil {
		return fmt.Errorf("sendDelta: marshal: %w", err)
	}

	url := platformURL + "/api/internal/streaming-delta"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("sendDelta: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := deltaHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("sendDelta: send: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("sendDelta: status %d", resp.StatusCode)
	}
	return nil
}

// ChatWithToolsStream mirrors ChatWithTools but uses streaming for text generation.
// Token deltas are batched (~50ms or 20 chars) and delivered via the onDelta hook.
// Tool calls are accumulated from streamed deltas and executed sequentially between
// streaming rounds, matching the non-streaming tool loop behavior.
func (c *OpenAIClient) ChatWithToolsStream(
	ctx context.Context,
	messages []chatMessage,
	tools []ToolDefinition,
	onToolCall ToolCallHook,
	onToolResult ToolResultHook,
	onDelta StreamDeltaHook,
) (string, []chatMessage, error) {
	// Build OpenAI tool definitions.
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
		reqBody := streamChatRequest{
			Model:             c.client.Model(),
			Messages:          history,
			Stream:            true,
			StreamOptions:     &streamOpts{IncludeUsage: true},
			ParallelToolCalls: &parallelCalls,
		}
		if len(apiTools) > 0 {
			reqBody.Tools = apiTools
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			return "", nil, fmt.Errorf("chatWithToolsStream: marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
		if err != nil {
			return "", nil, fmt.Errorf("chatWithToolsStream: create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.client.APIKey())

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", nil, fmt.Errorf("chatWithToolsStream: send request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return "", nil, fmt.Errorf("chatWithToolsStream: openai error (status %d): %s", resp.StatusCode, string(respBody))
		}

		// Process the streaming response.
		fullText, accumulators, finishReason, streamErr := c.processStream(resp.Body, onDelta)
		resp.Body.Close()
		if streamErr != nil {
			return "", nil, fmt.Errorf("chatWithToolsStream: %w", streamErr)
		}

		// Handle based on finish reason.
		if finishReason != "tool_calls" {
			// Text response complete -- signal done and return.
			if onDelta != nil {
				onDelta("", true)
			}
			history = append(history, chatMessage{
				Role:    "assistant",
				Content: fullText,
			})
			return fullText, history, nil
		}

		// Tool calls -- build tool call structs from accumulators.
		toolCalls := make([]toolCall, 0, len(accumulators))
		for i := 0; i < len(accumulators); i++ {
			acc, ok := accumulators[i]
			if !ok {
				continue
			}
			toolCalls = append(toolCalls, toolCall{
				ID:   acc.ID,
				Type: "function",
				Function: toolCallFunc{
					Name:      acc.Name,
					Arguments: acc.Arguments.String(),
				},
			})
		}

		// Append assistant message with tool_calls (no content).
		assistantMsg := chatMessage{
			Role:      "assistant",
			ToolCalls: toolCalls,
		}
		history = append(history, assistantMsg)

		// Execute each tool call sequentially (per Pitfall 10).
		for _, tc := range toolCalls {
			if onToolCall != nil {
				onToolCall(tc.Function.Name, tc.Function.Arguments)
			}

			tool, ok := toolMap[tc.Function.Name]
			var result string
			if !ok {
				result = fmt.Sprintf("Error: unknown tool '%s'", tc.Function.Name)
				log.Printf("chatWithToolsStream: unknown tool %q requested by model", tc.Function.Name)
			} else {
				var execErr error
				result, execErr = tool.Execute(ctx, json.RawMessage(tc.Function.Arguments))
				if execErr != nil {
					result = fmt.Sprintf("Tool error: %v", execErr)
					log.Printf("chatWithToolsStream: tool %q execution failed: %v", tc.Function.Name, execErr)
				}
			}

			if onToolResult != nil {
				summary := result
				if len(summary) > 100 {
					summary = summary[:100] + "..."
				}
				onToolResult(tc.Function.Name, summary)
			}

			// Append tool result message.
			history = append(history, chatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
			})
		}
		// Loop back to make another streaming request with updated history.
	}

	log.Printf("chatWithToolsStream: reached max tool call rounds (%d)", maxToolRounds)
	return "I was unable to complete the task within the allowed number of tool call iterations.", history, nil
}

// processStream reads an OpenAI SSE stream, batching text deltas and accumulating
// tool call arguments. Returns the full assembled text, tool call accumulators,
// the finish reason, and any error.
func (c *OpenAIClient) processStream(
	body io.Reader,
	onDelta StreamDeltaHook,
) (string, map[int]*toolCallAccumulator, string, error) {
	scanner := bufio.NewScanner(body)

	var fullText strings.Builder
	accumulators := make(map[int]*toolCallAccumulator)
	var finishReason string

	// Token batching: flush every ~50ms or 20 chars to reduce event volume.
	var batchBuf strings.Builder
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	flushBatch := func() {
		if batchBuf.Len() > 0 && onDelta != nil {
			onDelta(batchBuf.String(), false)
			batchBuf.Reset()
		}
	}

	for scanner.Scan() {
		line := scanner.Text()

		// SSE format: lines starting with "data: "
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			log.Printf("processStream: unmarshal chunk: %v", err)
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		// Text delta -- accumulate and batch for delivery.
		if choice.Delta.Content != "" {
			fullText.WriteString(choice.Delta.Content)
			batchBuf.WriteString(choice.Delta.Content)

			// Flush if batch exceeds 20 chars.
			if batchBuf.Len() >= 20 {
				flushBatch()
			}
		}

		// Tool call deltas -- accumulate by index (per Pitfall 2).
		for _, tc := range choice.Delta.ToolCalls {
			acc, ok := accumulators[tc.Index]
			if !ok {
				acc = &toolCallAccumulator{}
				accumulators[tc.Index] = acc
			}
			if tc.ID != "" {
				acc.ID = tc.ID
			}
			if tc.Function.Name != "" {
				acc.Name = tc.Function.Name
			}
			acc.Arguments.WriteString(tc.Function.Arguments)
		}

		// Check finish reason.
		if choice.FinishReason != nil {
			finishReason = *choice.FinishReason
		}

		// Check ticker for time-based flush.
		select {
		case <-ticker.C:
			flushBatch()
		default:
		}
	}

	// Flush any remaining batched text.
	flushBatch()

	if err := scanner.Err(); err != nil {
		return "", nil, "", fmt.Errorf("processStream: scanner error: %w", err)
	}

	return fullText.String(), accumulators, finishReason, nil
}
