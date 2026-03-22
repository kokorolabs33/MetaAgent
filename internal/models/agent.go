package models

import (
	"encoding/json"
	"time"
)

type Agent struct {
	ID            string          `json:"id"`
	OrgID         string          `json:"org_id"`
	Name          string          `json:"name"`
	Version       string          `json:"version"`
	Description   string          `json:"description"`
	Endpoint      string          `json:"endpoint"`
	AdapterType   string          `json:"adapter_type"`
	AdapterConfig json.RawMessage `json:"adapter_config,omitempty"`
	AuthType      string          `json:"auth_type"`
	AuthConfig    json.RawMessage `json:"auth_config,omitempty"` // encrypted at rest
	Capabilities  []string        `json:"capabilities"`
	InputSchema   json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema  json.RawMessage `json:"output_schema,omitempty"`
	Config        json.RawMessage `json:"config,omitempty"`
	Status        string          `json:"status"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// HTTPPollConfig is the typed form of adapter_config for http_poll agents.
type HTTPPollConfig struct {
	Submit    HTTPPollSubmit     `json:"submit"`
	Poll      HTTPPollPoll       `json:"poll"`
	SendInput *HTTPPollSendInput `json:"send_input,omitempty"`
}

type HTTPPollSubmit struct {
	Method       string         `json:"method"`
	Path         string         `json:"path"`
	BodyTemplate map[string]any `json:"body_template,omitempty"`
	JobIDPath    string         `json:"job_id_path"`
}

type HTTPPollPoll struct {
	Method          string            `json:"method"`
	Path            string            `json:"path"`
	IntervalSeconds int               `json:"interval_seconds"`
	StatusPath      string            `json:"status_path"`
	StatusMap       map[string]string `json:"status_map,omitempty"`
	ResultPath      string            `json:"result_path,omitempty"`
	ErrorPath       string            `json:"error_path,omitempty"`
	MessagesPath    string            `json:"messages_path,omitempty"`
	ProgressPath    string            `json:"progress_path,omitempty"`
}

type HTTPPollSendInput struct {
	Method       string         `json:"method"`
	Path         string         `json:"path"`
	BodyTemplate map[string]any `json:"body_template,omitempty"`
}

// AgentAuthConfig holds the decrypted auth configuration.
type AgentAuthConfig struct {
	Token  string `json:"token,omitempty"`  // for bearer
	Key    string `json:"key,omitempty"`    // for api_key
	Header string `json:"header,omitempty"` // for api_key (custom header name)
	User   string `json:"user,omitempty"`   // for basic
	Pass   string `json:"pass,omitempty"`   // for basic
}
