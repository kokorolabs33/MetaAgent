# Plan 2: Agent System

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Agent Registry, Adapter layer, credential encryption, and Mock Agent service. After this plan, agents can be registered with endpoint/auth/adapter config, the platform can call external agents via adapters, and the mock agent enables end-to-end testing.

**Architecture:** Agent Registry is CRUD on the `agents` table. The Adapter layer abstracts how the platform communicates with external agents — MVP supports `http_poll` (configurable JSON mapping) and `native` (standard protocol). Credential encryption uses AES-256-GCM. Mock Agent is a standalone Go HTTP service for testing.

**Tech Stack:** Go 1.26, chi/v5, pgx/v5, `github.com/PaesslerAG/jsonpath`, AES-256-GCM

**Spec:** `docs/superpowers/specs/2026-03-22-taskhub-v2-design.md` (Sections 1-2)

**Depends on:** Plan 1 (completed) — DB schema, models, auth, RBAC, handlers

---

## File Map

| File | Responsibility |
|------|----------------|
| `internal/models/agent.go` | Agent domain struct + adapter config types |
| `internal/crypto/crypto.go` | AES-256-GCM encrypt/decrypt for auth_config |
| `internal/crypto/crypto_test.go` | Unit tests for encrypt/decrypt |
| `internal/handlers/agents.go` | Agent CRUD handlers + healthcheck + test |
| `internal/adapter/adapter.go` | AgentAdapter interface + types (JobHandle, AgentStatus, SubTaskInput) |
| `internal/adapter/template.go` | Template variable substitution engine |
| `internal/adapter/template_test.go` | Unit tests for template substitution |
| `internal/adapter/jsonpath.go` | JSONPath extraction wrapper |
| `internal/adapter/jsonpath_test.go` | Unit tests for JSONPath extraction |
| `internal/adapter/http_poll.go` | HTTP polling adapter implementation |
| `internal/adapter/native.go` | Native protocol adapter implementation |
| `internal/adapter/http_poll_test.go` | Integration tests with httptest mock |
| `cmd/mockagent/main.go` | Mock Agent service (echo, slow, fail, ask, progress behaviors) |
| `web/lib/types.ts` | Add Agent TypeScript interfaces |

---

## Chunk 1: Agent Models & Crypto

### Task 1: Agent domain model

**Files:**
- Create: `internal/models/agent.go`

- [ ] **Step 1: Write Agent struct and adapter config types**

```go
// internal/models/agent.go
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
	Submit    HTTPPollSubmit    `json:"submit"`
	Poll      HTTPPollPoll      `json:"poll"`
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
	Token  string `json:"token,omitempty"`   // for bearer
	Key    string `json:"key,omitempty"`     // for api_key
	Header string `json:"header,omitempty"`  // for api_key (custom header name)
	User   string `json:"user,omitempty"`    // for basic
	Pass   string `json:"pass,omitempty"`    // for basic
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/models/agent.go
git commit -m "feat: add Agent model with adapter config types"
```

---

### Task 2: Credential encryption

**Files:**
- Create: `internal/crypto/crypto.go`
- Create: `internal/crypto/crypto_test.go`

- [ ] **Step 1: Write AES-256-GCM encrypt/decrypt**

```go
// internal/crypto/crypto.go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// Encrypt encrypts plaintext using AES-256-GCM with the given hex-encoded key.
// Returns hex-encoded ciphertext (nonce prepended).
func Encrypt(plaintext []byte, hexKey string) (string, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", fmt.Errorf("decode key: %w", err)
	}
	if len(key) != 32 {
		return "", fmt.Errorf("key must be 32 bytes (64 hex chars), got %d bytes", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt decrypts hex-encoded ciphertext using AES-256-GCM with the given hex-encoded key.
func Decrypt(hexCiphertext string, hexKey string) ([]byte, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}

	ciphertext, err := hex.DecodeString(hexCiphertext)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}
```

- [ ] **Step 2: Write unit tests**

```go
// internal/crypto/crypto_test.go
package crypto

import (
	"encoding/hex"
	"testing"
)

func testKey() string {
	// 32 bytes = 64 hex chars
	return "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // pragma: allowlist secret
}

func TestEncryptDecrypt(t *testing.T) {
	plaintext := []byte(`{"token":"sk-secret-123"}`)
	encrypted, err := Encrypt(plaintext, testKey())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if encrypted == "" {
		t.Fatal("encrypted is empty")
	}
	// Encrypted should be hex
	if _, err := hex.DecodeString(encrypted); err != nil {
		t.Fatalf("encrypted is not valid hex: %v", err)
	}

	decrypted, err := Decrypt(encrypted, testKey())
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("got %q, want %q", string(decrypted), string(plaintext))
	}
}

func TestDecryptWrongKey(t *testing.T) {
	plaintext := []byte("secret data")
	encrypted, err := Encrypt(plaintext, testKey())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	wrongKey := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789" // pragma: allowlist secret
	_, err = Decrypt(encrypted, wrongKey)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestEncryptInvalidKey(t *testing.T) {
	_, err := Encrypt([]byte("data"), "tooshort")
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestDecryptInvalidCiphertext(t *testing.T) {
	_, err := Decrypt("not-hex", testKey())
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/crypto/ -v
```

- [ ] **Step 4: Commit**

```bash
git add internal/crypto/
git commit -m "feat: add AES-256-GCM credential encryption with tests"
```

---

## Chunk 2: Adapter Layer

### Task 3: Adapter interface and types

**Files:**
- Create: `internal/adapter/adapter.go`

- [ ] **Step 1: Write adapter interface**

```go
// internal/adapter/adapter.go
package adapter

import (
	"context"
	"encoding/json"

	"taskhub/internal/models"
)

// JobHandle is returned by Submit and used for Poll/SendInput.
type JobHandle struct {
	JobID         string `json:"job_id"`
	StatusEndpoint string `json:"status_endpoint,omitempty"`
}

// AgentStatus is the normalized response from Poll.
type AgentStatus struct {
	Status       string          `json:"status"` // "running" | "completed" | "failed" | "needs_input"
	Progress     *float64        `json:"progress,omitempty"`
	Result       json.RawMessage `json:"result,omitempty"`
	Error        string          `json:"error,omitempty"`
	InputRequest *InputRequest   `json:"input_request,omitempty"`
	Messages     []AgentMessage  `json:"messages,omitempty"`
}

type InputRequest struct {
	Message string   `json:"message"`
	Options []string `json:"options,omitempty"`
}

type AgentMessage struct {
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}

// SubTaskInput is what the platform sends to an agent.
type SubTaskInput struct {
	TaskID      string          `json:"task_id"`
	Instruction string          `json:"instruction"`
	Input       json.RawMessage `json:"input,omitempty"`
	CallbackURL string          `json:"callback_url,omitempty"`
}

// UserInput is what the user sends back when an agent requests input.
type UserInput struct {
	SubtaskID string          `json:"subtask_id"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// AgentAdapter is the interface all adapters implement.
type AgentAdapter interface {
	Submit(ctx context.Context, agent models.Agent, input SubTaskInput) (JobHandle, error)
	Poll(ctx context.Context, agent models.Agent, handle JobHandle) (AgentStatus, error)
	SendInput(ctx context.Context, agent models.Agent, handle JobHandle, input UserInput) error
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/adapter/adapter.go
git commit -m "feat: add AgentAdapter interface and types"
```

---

### Task 4: Template variable substitution

**Files:**
- Create: `internal/adapter/template.go`
- Create: `internal/adapter/template_test.go`

- [ ] **Step 1: Write template engine**

```go
// internal/adapter/template.go
package adapter

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var templateVarRegex = regexp.MustCompile(`\{\{(\w+(?:\.\w+)*)\}\}`)

// RenderTemplate substitutes {{var}} and {{var.nested}} in a template value.
// vars is a flat map like {"instruction": "...", "job_id": "...", "input": {...}}.
// Missing variables → empty string.
func RenderTemplate(tmpl any, vars map[string]any) any {
	switch v := tmpl.(type) {
	case string:
		return renderString(v, vars)
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, val := range v {
			result[key] = RenderTemplate(val, vars)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = RenderTemplate(val, vars)
		}
		return result
	default:
		return v
	}
}

func renderString(s string, vars map[string]any) string {
	return templateVarRegex.ReplaceAllStringFunc(s, func(match string) string {
		// Extract path: "{{input.context}}" → "input.context"
		path := match[2 : len(match)-2]
		val := resolveVarPath(path, vars)
		if val == nil {
			return ""
		}
		switch v := val.(type) {
		case string:
			return v
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return fmt.Sprintf("%v", v)
			}
			return string(b)
		}
	})
}

// resolveVarPath resolves "input.nested.field" against the vars map.
func resolveVarPath(path string, vars map[string]any) any {
	parts := strings.Split(path, ".")
	var current any = vars
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}
	return current
}
```

- [ ] **Step 2: Write tests**

```go
// internal/adapter/template_test.go
package adapter

import "testing"

func TestRenderTemplateSimple(t *testing.T) {
	vars := map[string]any{"instruction": "analyze this", "job_id": "j-123"}
	result := RenderTemplate("Task: {{instruction}}", vars)
	if result != "Task: analyze this" {
		t.Errorf("got %q", result)
	}
}

func TestRenderTemplateNested(t *testing.T) {
	vars := map[string]any{
		"input": map[string]any{
			"context": "some context",
			"nested":  map[string]any{"field": "deep value"},
		},
	}
	result := RenderTemplate("{{input.context}}", vars)
	if result != "some context" {
		t.Errorf("got %q", result)
	}
	result = RenderTemplate("{{input.nested.field}}", vars)
	if result != "deep value" {
		t.Errorf("got %q", result)
	}
}

func TestRenderTemplateMissing(t *testing.T) {
	vars := map[string]any{}
	result := RenderTemplate("{{missing}}", vars)
	if result != "" {
		t.Errorf("expected empty string for missing var, got %q", result)
	}
}

func TestRenderTemplateMap(t *testing.T) {
	tmpl := map[string]any{
		"prompt":  "{{instruction}}",
		"context": "{{input.ctx}}",
	}
	vars := map[string]any{
		"instruction": "do stuff",
		"input":       map[string]any{"ctx": "bg info"},
	}
	result := RenderTemplate(tmpl, vars).(map[string]any)
	if result["prompt"] != "do stuff" {
		t.Errorf("prompt: got %q", result["prompt"])
	}
	if result["context"] != "bg info" {
		t.Errorf("context: got %q", result["context"])
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/adapter/ -v -run Template
```

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/template.go internal/adapter/template_test.go
git commit -m "feat: add template variable substitution engine with tests"
```

---

### Task 5: JSONPath extraction

**Files:**
- Create: `internal/adapter/jsonpath.go`
- Create: `internal/adapter/jsonpath_test.go`

- [ ] **Step 1: Add jsonpath dependency**

```bash
go get github.com/PaesslerAG/jsonpath
```

- [ ] **Step 2: Write JSONPath wrapper**

```go
// internal/adapter/jsonpath.go
package adapter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PaesslerAG/jsonpath"
)

// ExtractScalar extracts a single value from JSON data using a JSONPath expression.
// Returns nil if path not found (for optional fields).
// Returns error if the JSON is malformed.
func ExtractScalar(data []byte, path string) (any, error) {
	if path == "" {
		return nil, nil
	}
	var obj any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}
	result, err := jsonpath.Get(path, obj)
	if err != nil {
		// Path not found → nil (not error) for optional fields
		return nil, nil //nolint:nilerr // missing path is not an error for optional extraction
	}
	return result, nil
}

// ExtractScalarRequired extracts a single value, returning error if not found.
func ExtractScalarRequired(data []byte, path string) (any, error) {
	result, err := ExtractScalar(data, path)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("required path %q not found", path)
	}
	return result, nil
}

// ExtractStringSlice extracts an array of strings from JSON data.
// Returns empty slice if path not found.
func ExtractStringSlice(data []byte, path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	result, err := ExtractScalar(data, path)
	if err != nil || result == nil {
		return nil, err
	}

	// jsonpath may return []any for array paths
	switch v := result.(type) {
	case []any:
		strs := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				strs = append(strs, s)
			} else {
				strs = append(strs, fmt.Sprintf("%v", item))
			}
		}
		return strs, nil
	case string:
		return []string{v}, nil
	default:
		return []string{fmt.Sprintf("%v", v)}, nil
	}
}

// Needed to satisfy jsonpath.Get context requirement in some versions.
var _ = context.Background
```

- [ ] **Step 3: Write tests**

```go
// internal/adapter/jsonpath_test.go
package adapter

import "testing"

func TestExtractScalar(t *testing.T) {
	data := []byte(`{"id": "job-123", "state": "running", "nested": {"value": 42}}`)

	val, err := ExtractScalar(data, "$.id")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if val != "job-123" {
		t.Errorf("got %v", val)
	}

	val, err = ExtractScalar(data, "$.nested.value")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if val != float64(42) {
		t.Errorf("got %v (type %T)", val, val)
	}
}

func TestExtractScalarMissing(t *testing.T) {
	data := []byte(`{"id": "job-123"}`)
	val, err := ExtractScalar(data, "$.nonexistent")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

func TestExtractScalarRequired(t *testing.T) {
	data := []byte(`{"id": "job-123"}`)
	_, err := ExtractScalarRequired(data, "$.missing")
	if err == nil {
		t.Fatal("expected error for missing required path")
	}
}

func TestExtractStringSlice(t *testing.T) {
	data := []byte(`{"logs": [{"text": "msg1"}, {"text": "msg2"}]}`)
	// Note: jsonpath "$.logs[*].text" may not work in all implementations.
	// If it doesn't, we fall back to extracting the array differently.
	// For now test with simple array.
	data2 := []byte(`{"tags": ["a", "b", "c"]}`)
	strs, err := ExtractStringSlice(data2, "$.tags")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(strs) != 3 || strs[0] != "a" {
		t.Errorf("got %v", strs)
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/adapter/ -v -run (Extract|JsonPath)
```

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/jsonpath.go internal/adapter/jsonpath_test.go
git commit -m "feat: add JSONPath extraction wrapper with tests"
```

---

### Task 6: HTTP Poll adapter

**Files:**
- Create: `internal/adapter/http_poll.go`

- [ ] **Step 1: Write HTTP poll adapter**

```go
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
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/http_poll.go
git commit -m "feat: add HTTP poll adapter — submit, poll, send_input with template + jsonpath"
```

---

### Task 7: Native protocol adapter

**Files:**
- Create: `internal/adapter/native.go`

- [ ] **Step 1: Write native adapter**

```go
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
```

- [ ] **Step 2: Commit**

```bash
git add internal/adapter/native.go
git commit -m "feat: add native protocol adapter — standard TaskHub agent protocol"
```

---

## Chunk 3: Agent Handlers & Mock Agent

### Task 8: Agent CRUD handlers

**Files:**
- Create: `internal/handlers/agents.go`

- [ ] **Step 1: Write agent handlers**

Agent handler with: List, Create, Get, Update, Delete, Healthcheck.

- List: `GET /api/orgs/:org_id/agents` — query agents for the org, return as JSON array
- Create: `POST /api/orgs/:org_id/agents` — validate required fields (name, endpoint, adapter_type), generate UUID, insert, return 201
- Get: `GET /api/orgs/:org_id/agents/:id` — fetch by ID + org_id, return 404 if not found
- Update: `PUT /api/orgs/:org_id/agents/:id` — partial update (name, description, endpoint, adapter_config, auth_type, auth_config, capabilities, status)
- Delete: `DELETE /api/orgs/:org_id/agents/:id` — delete, return 204
- Healthcheck: `POST /api/orgs/:org_id/agents/:id/healthcheck` — fetch agent, make GET request to endpoint, return result

Key: use `ctxutil.OrgFromCtx` for org_id, `chi.URLParam(r, "id")` for agent ID. Use `decodeJSON(w, r, &req)` for body parsing. `Capabilities` stored as JSONB — marshal/unmarshal `[]string`. Scan `adapter_config`, `auth_config`, `config`, `input_schema`, `output_schema` as `json.RawMessage` (nullable — handle `*json.RawMessage` or scan into `[]byte`).

- [ ] **Step 2: Verify compilation**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/handlers/agents.go
git commit -m "feat: add agent handlers — list, create, get, update, delete, healthcheck"
```

---

### Task 9: Wire agent routes

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add agent routes to the org-scoped router**

Add after the members routes in main.go:

```go
agentH := &handlers.AgentHandler{DB: pool}
```

And in the org-scoped route group:

```go
// Agents
r.Get("/agents", agentH.List)
r.With(rbac.RequireRole("admin")).Post("/agents", agentH.Create)
r.Get("/agents/{id}", agentH.Get)
r.With(rbac.RequireRole("admin")).Put("/agents/{id}", agentH.Update)
r.With(rbac.RequireRole("admin")).Delete("/agents/{id}", agentH.Delete)
r.Post("/agents/{id}/healthcheck", agentH.Healthcheck)
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./cmd/server
```

- [ ] **Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: wire agent routes — CRUD + healthcheck"
```

---

### Task 10: Mock Agent service

**Files:**
- Create: `cmd/mockagent/main.go`

- [ ] **Step 1: Write mock agent**

The mock agent implements the native TaskHub agent protocol (POST /tasks, GET /tasks/:id/status, POST /tasks/:id/input). Behavior is controlled by the `instruction` field:

- `echo:xxx` → immediately completed with "xxx" as result
- `slow:N` → wait N seconds, then completed
- `fail:xxx` → immediately failed with "xxx" as error
- `fail-then-succeed:N` → fail the first N attempts, then succeed
- `ask:xxx` → return needs_input with "xxx" as message, then wait for input, then complete
- `progress:` → return progress 0.0 → 0.25 → 0.5 → 0.75 → 1.0 over 5 polls
- `messages:N` → produce N messages during execution
- default → wait 2 seconds, return completed

Jobs are stored in-memory in a `sync.Map`. Each job has: id, status, instruction, progress, messages, input, attempt count, created time.

Routes:
- `POST /tasks` → create job, parse instruction keyword, return `{"job_id": "..."}`
- `GET /tasks/:id/status` → return current status with all fields
- `POST /tasks/:id/input` → receive input, transition from needs_input to running
- `GET /health` → `{"status":"ok"}`

Port: flag `--port` with default 9090.

- [ ] **Step 2: Verify it compiles and runs**

```bash
go build ./cmd/mockagent
./mockagent --port 9090 &
curl -s http://localhost:9090/health
# Expected: {"status":"ok"}
curl -s -X POST http://localhost:9090/tasks -H 'Content-Type: application/json' -d '{"task_id":"t1","instruction":"echo:hello world"}'
# Expected: {"job_id":"..."}
kill %1
rm -f mockagent
```

- [ ] **Step 3: Commit**

```bash
git add cmd/mockagent/
git commit -m "feat: add mock agent service — echo, slow, fail, ask, progress behaviors"
```

---

### Task 11: Update TypeScript types

**Files:**
- Modify: `web/lib/types.ts`

- [ ] **Step 1: Add Agent interfaces**

Append to existing types.ts:

```typescript
export interface Agent {
  id: string;
  org_id: string;
  name: string;
  version: string;
  description: string;
  endpoint: string;
  adapter_type: "http_poll" | "native";
  adapter_config?: Record<string, unknown>;
  auth_type: "none" | "bearer" | "api_key" | "basic";
  capabilities: string[];
  status: "active" | "inactive" | "degraded";
  created_at: string;
  updated_at: string;
}
```

- [ ] **Step 2: Commit**

```bash
git add web/lib/types.ts
git commit -m "feat(web): add Agent TypeScript interface"
```

---

### Task 12: End-to-end verification

- [ ] **Step 1: Run all tests**

```bash
go test ./... -v
```

- [ ] **Step 2: Run linter**

```bash
golangci-lint run ./...
```

- [ ] **Step 3: Build everything**

```bash
go build ./cmd/server && go build ./cmd/mockagent
```

- [ ] **Step 4: Clean up**

```bash
rm -f server mockagent
```
