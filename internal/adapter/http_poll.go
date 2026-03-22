// internal/adapter/http_poll.go
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

// HTTPPollAdapter implements AgentAdapter for http_poll agents.
type HTTPPollAdapter struct {
	Client *http.Client
}

func NewHTTPPollAdapter() *HTTPPollAdapter {
	return &HTTPPollAdapter{Client: &http.Client{}}
}

func (a *HTTPPollAdapter) Submit(ctx context.Context, agent models.Agent, input SubTaskInput) (JobHandle, error) {
	cfg, err := parseHTTPPollConfig(agent.AdapterConfig)
	if err != nil {
		return JobHandle{}, fmt.Errorf("parse adapter config: %w", err)
	}

	// Build request body
	vars := map[string]any{
		"instruction": input.Instruction,
		"task_id":     input.TaskID,
	}
	if input.Input != nil {
		var inputMap map[string]any
		if err := json.Unmarshal(input.Input, &inputMap); err == nil {
			vars["input"] = inputMap
		}
	}

	var body any
	if cfg.Submit.BodyTemplate != nil {
		body = RenderTemplate(cfg.Submit.BodyTemplate, vars)
	} else {
		body = input // default: send the whole SubTaskInput
	}

	url := agent.Endpoint + renderString(cfg.Submit.Path, vars)
	respBody, err := a.doRequest(ctx, agent, cfg.Submit.Method, url, body)
	if err != nil {
		return JobHandle{}, fmt.Errorf("submit: %w", err)
	}

	// Extract job_id
	jobIDRaw, err := ExtractScalarRequired(respBody, cfg.Submit.JobIDPath)
	if err != nil {
		return JobHandle{}, fmt.Errorf("extract job_id: %w", err)
	}
	jobID := fmt.Sprintf("%v", jobIDRaw)

	// Extract optional status_endpoint
	handle := JobHandle{JobID: jobID}
	return handle, nil
}

func (a *HTTPPollAdapter) Poll(ctx context.Context, agent models.Agent, handle JobHandle) (AgentStatus, error) {
	cfg, err := parseHTTPPollConfig(agent.AdapterConfig)
	if err != nil {
		return AgentStatus{}, fmt.Errorf("parse adapter config: %w", err)
	}

	vars := map[string]any{"job_id": handle.JobID}
	url := agent.Endpoint + renderString(cfg.Poll.Path, vars)

	respBody, err := a.doRequest(ctx, agent, cfg.Poll.Method, url, nil)
	if err != nil {
		return AgentStatus{}, fmt.Errorf("poll: %w", err)
	}

	// Extract status (required)
	statusRaw, err := ExtractScalarRequired(respBody, cfg.Poll.StatusPath)
	if err != nil {
		return AgentStatus{}, fmt.Errorf("extract status: %w", err)
	}
	rawStatus := fmt.Sprintf("%v", statusRaw)

	// Map status if status_map is configured
	status := rawStatus
	if cfg.Poll.StatusMap != nil {
		if mapped, ok := cfg.Poll.StatusMap[rawStatus]; ok {
			status = mapped
		}
	}

	result := AgentStatus{Status: status}

	// Extract optional fields
	if cfg.Poll.ResultPath != "" {
		if r, _ := ExtractScalar(respBody, cfg.Poll.ResultPath); r != nil {
			b, _ := json.Marshal(r)
			result.Result = b
		}
	}
	if cfg.Poll.ErrorPath != "" {
		if e, _ := ExtractScalar(respBody, cfg.Poll.ErrorPath); e != nil {
			result.Error = fmt.Sprintf("%v", e)
		}
	}
	if cfg.Poll.ProgressPath != "" {
		if p, _ := ExtractScalar(respBody, cfg.Poll.ProgressPath); p != nil {
			if pf, ok := p.(float64); ok {
				result.Progress = &pf
			}
		}
	}
	if cfg.Poll.MessagesPath != "" {
		if msgs, _ := ExtractStringSlice(respBody, cfg.Poll.MessagesPath); msgs != nil {
			for _, m := range msgs {
				result.Messages = append(result.Messages, AgentMessage{Content: m})
			}
		}
	}

	return result, nil
}

func (a *HTTPPollAdapter) SendInput(ctx context.Context, agent models.Agent, handle JobHandle, input UserInput) error {
	cfg, err := parseHTTPPollConfig(agent.AdapterConfig)
	if err != nil {
		return fmt.Errorf("parse adapter config: %w", err)
	}
	if cfg.SendInput == nil {
		return fmt.Errorf("agent does not support send_input")
	}

	vars := map[string]any{
		"job_id":  handle.JobID,
		"message": input.Message,
	}

	var body any
	if cfg.SendInput.BodyTemplate != nil {
		body = RenderTemplate(cfg.SendInput.BodyTemplate, vars)
	} else {
		body = input
	}

	url := agent.Endpoint + renderString(cfg.SendInput.Path, vars)
	_, err = a.doRequest(ctx, agent, cfg.SendInput.Method, url, body)
	return err
}

func (a *HTTPPollAdapter) doRequest(ctx context.Context, agent models.Agent, method, url string, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply auth
	applyAuth(req, agent)

	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("agent returned %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func applyAuth(req *http.Request, agent models.Agent) {
	if agent.AuthType == "none" || agent.AuthConfig == nil {
		return
	}
	var auth models.AgentAuthConfig
	if err := json.Unmarshal(agent.AuthConfig, &auth); err != nil {
		return
	}
	switch agent.AuthType {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+auth.Token)
	case "api_key":
		header := auth.Header
		if header == "" {
			header = "X-API-Key"
		}
		req.Header.Set(header, auth.Key)
	case "basic":
		req.SetBasicAuth(auth.User, auth.Pass)
	}
}

func parseHTTPPollConfig(raw json.RawMessage) (*models.HTTPPollConfig, error) {
	var cfg models.HTTPPollConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal http_poll config: %w", err)
	}
	return &cfg, nil
}
