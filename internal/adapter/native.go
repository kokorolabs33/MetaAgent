// internal/adapter/native.go
package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"taskhub/internal/models"
)

// NativeAdapter implements AgentAdapter for agents using the standard TaskHub protocol.
// Expects: POST /tasks, GET /tasks/{job_id}/status, POST /tasks/{job_id}/input
type NativeAdapter struct {
	Client *http.Client
}

func NewNativeAdapter() *NativeAdapter {
	return &NativeAdapter{Client: &http.Client{}}
}

func (a *NativeAdapter) Submit(ctx context.Context, agent models.Agent, input SubTaskInput) (JobHandle, error) {
	url := agent.Endpoint + "/tasks"
	body, _ := json.Marshal(input)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return JobHandle{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	applyAuth(req, agent)

	resp, err := a.Client.Do(req)
	if err != nil {
		return JobHandle{}, fmt.Errorf("submit: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return JobHandle{}, fmt.Errorf("agent returned %d: %s", resp.StatusCode, string(respBody))
	}

	var handle JobHandle
	if err := json.Unmarshal(respBody, &handle); err != nil {
		return JobHandle{}, fmt.Errorf("unmarshal response: %w", err)
	}
	return handle, nil
}

func (a *NativeAdapter) Poll(ctx context.Context, agent models.Agent, handle JobHandle) (AgentStatus, error) {
	url := agent.Endpoint + "/tasks/" + handle.JobID + "/status"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return AgentStatus{}, err
	}
	applyAuth(req, agent)

	resp, err := a.Client.Do(req)
	if err != nil {
		return AgentStatus{}, fmt.Errorf("poll: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return AgentStatus{}, fmt.Errorf("agent returned %d: %s", resp.StatusCode, string(respBody))
	}

	var status AgentStatus
	if err := json.Unmarshal(respBody, &status); err != nil {
		return AgentStatus{}, fmt.Errorf("unmarshal status: %w", err)
	}
	return status, nil
}

func (a *NativeAdapter) SendInput(ctx context.Context, agent models.Agent, handle JobHandle, input UserInput) error {
	url := agent.Endpoint + "/tasks/" + handle.JobID + "/input"
	body, _ := json.Marshal(input)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	applyAuth(req, agent)

	resp, err := a.Client.Do(req)
	if err != nil {
		return fmt.Errorf("send input: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agent returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
