// Package a2a implements an A2A (Agent-to-Agent) protocol client using net/http.
// It sends JSON-RPC 2.0 requests to A2A-compliant agent servers.
package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

// Client is a lightweight A2A JSON-RPC client.
type Client struct {
	httpClient *http.Client
	idCounter  atomic.Int64
}

// NewClient creates a new A2A client with sensible defaults.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 5 * time.Minute, // generous timeout for LLM agents
		},
	}
}

// SendResult is the normalized result from a SendMessage call.
type SendResult struct {
	TaskID    string          // A2A task ID (server-generated)
	State     string          // "completed", "failed", "input-required", "working"
	Artifacts json.RawMessage // combined artifact data (JSON)
	Message   string          // status message (for input-required or error)
	Error     string          // error message if failed
}

// jsonRPCRequest is the JSON-RPC 2.0 request envelope.
type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	ID      string `json:"id"`
	Params  any    `json:"params"`
}

// jsonRPCResponse is the JSON-RPC 2.0 response envelope.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// a2aTask is the task object returned in JSON-RPC results.
type a2aTask struct {
	ID        string     `json:"id"`
	ContextID string     `json:"contextId,omitempty"`
	Status    a2aStatus  `json:"status"`
	Artifacts []artifact `json:"artifacts,omitempty"`
}

type a2aStatus struct {
	State   string      `json:"state"`
	Message *a2aMessage `json:"message,omitempty"`
}

type a2aMessage struct {
	Role  string        `json:"role"`
	Parts []MessagePart `json:"parts"`
}

// MessagePart is a part of an A2A message (text or structured data).
type MessagePart struct {
	Text string `json:"text,omitempty"`
	Data any    `json:"data,omitempty"`
}

type artifact struct {
	ArtifactID string        `json:"artifactId,omitempty"`
	Parts      []MessagePart `json:"parts,omitempty"`
}

// sendMessageParams is the params for the SendMessage JSON-RPC method.
type sendMessageParams struct {
	Message   a2aMessage `json:"message"`
	ContextID string     `json:"contextId,omitempty"`
	TaskID    string     `json:"taskId,omitempty"`
}

// taskIDParams is the params for GetTask and CancelTask JSON-RPC methods.
type taskIDParams struct {
	ID string `json:"id"`
}

// SendMessage sends a message to an A2A agent and returns the result.
func (c *Client) SendMessage(ctx context.Context, agentURL, contextID, taskID string, parts []MessagePart) (*SendResult, error) {
	msg := a2aMessage{
		Role:  "user",
		Parts: parts,
	}

	params := sendMessageParams{
		Message:   msg,
		ContextID: contextID,
		TaskID:    taskID,
	}

	resp, err := c.call(ctx, agentURL, "message/send", params)
	if err != nil {
		return nil, err
	}

	return c.parseTaskResult(resp)
}

// GetTask retrieves the current state of a task (for crash recovery).
func (c *Client) GetTask(ctx context.Context, agentURL, taskID string) (*SendResult, error) {
	params := taskIDParams{ID: taskID}

	resp, err := c.call(ctx, agentURL, "tasks/get", params)
	if err != nil {
		return nil, err
	}

	return c.parseTaskResult(resp)
}

// CancelTask requests cancellation of a task.
func (c *Client) CancelTask(ctx context.Context, agentURL, taskID string) error {
	params := taskIDParams{ID: taskID}

	_, err := c.call(ctx, agentURL, "tasks/cancel", params)
	return err
}

// TextPart creates a text message part.
func TextPart(text string) MessagePart {
	return MessagePart{Text: text}
}

// DataPart creates a data message part.
func DataPart(data any) MessagePart {
	return MessagePart{Data: data}
}

// call sends a JSON-RPC 2.0 request and returns the raw result.
func (c *Client) call(ctx context.Context, agentURL, method string, params any) (json.RawMessage, error) {
	id := fmt.Sprintf("%d", c.idCounter.Add(1))

	reqBody := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		ID:      id,
		Params:  params,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, agentURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request to %s: %w", agentURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("agent error (code %d): %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// parseTaskResult converts a raw JSON-RPC result into a SendResult.
func (c *Client) parseTaskResult(raw json.RawMessage) (*SendResult, error) {
	var task a2aTask
	if err := json.Unmarshal(raw, &task); err != nil {
		return nil, fmt.Errorf("unmarshal task result: %w", err)
	}

	result := &SendResult{
		TaskID: task.ID,
		State:  task.Status.State,
	}

	// Extract status message
	if task.Status.Message != nil && len(task.Status.Message.Parts) > 0 {
		for _, p := range task.Status.Message.Parts {
			if p.Text != "" {
				result.Message = p.Text
				break
			}
		}
	}

	// Extract artifacts
	if len(task.Artifacts) > 0 {
		artifactData := make([]any, 0, len(task.Artifacts))
		for _, art := range task.Artifacts {
			for _, part := range art.Parts {
				if part.Data != nil {
					artifactData = append(artifactData, part.Data)
				} else if part.Text != "" {
					artifactData = append(artifactData, part.Text)
				}
			}
		}
		if len(artifactData) == 1 {
			b, err := json.Marshal(artifactData[0])
			if err != nil {
				log.Printf("a2a: marshal artifact: %v", err)
				result.Artifacts = nil
				result.Error = fmt.Sprintf("marshal artifact: %v", err)
			} else {
				result.Artifacts = b
			}
		} else if len(artifactData) > 1 {
			b, err := json.Marshal(artifactData)
			if err != nil {
				log.Printf("a2a: marshal artifacts: %v", err)
				result.Artifacts = nil
				result.Error = fmt.Sprintf("marshal artifacts: %v", err)
			} else {
				result.Artifacts = b
			}
		}
	}

	if task.Status.State == "failed" && result.Message != "" {
		result.Error = result.Message
	}

	return result, nil
}
