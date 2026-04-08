# Deal Review LLM Agents — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build 4 real LLM-powered agents (Legal, Finance, Technical, Deal Review) as A2A servers with Claude API, deployed via Docker Compose.

**Architecture:** Single Go binary `cmd/llmagent` with `--role` flag. Each role has a different system prompt and AgentCard. Uses Anthropic Messages API directly (HTTP). All agents implement A2A server protocol using `a2a-go/v2` SDK.

**Tech Stack:** Go 1.26, `github.com/a2aproject/a2a-go/v2`, Anthropic Messages API, Docker

**Spec:** `docs/superpowers/specs/2026-03-23-a2a-protocol-deal-review-agents.md`

**Prerequisite:** Plan 1 (A2A Platform Migration) must be completed first.

---

## File Structure

### Files to Create

| Path | Responsibility |
|------|---------------|
| `cmd/llmagent/main.go` | A2A server entry point, role routing, CLI flags |
| `cmd/llmagent/roles.go` | Role definitions: system prompts, AgentCard config per role |
| `cmd/llmagent/executor.go` | `AgentExecutor` implementation: message → Claude API → response |
| `cmd/llmagent/claude.go` | Claude Messages API HTTP client |
| `cmd/llmagent/Dockerfile` | Multi-stage Docker build for LLM agent |
| `docker-compose.agents.yml` | 4 agent containers + TaskHub backend |

---

### Task 1: Claude API Client

**Files:**
- Create: `cmd/llmagent/claude.go`

- [ ] **Step 1: Create the Claude Messages API client**

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const claudeAPIURL = "https://api.anthropic.com/v1/messages"

// ClaudeClient calls the Anthropic Messages API.
type ClaudeClient struct {
	APIKey     string
	Model      string
	MaxTokens  int
	HTTPClient *http.Client
}

// NewClaudeClient creates a client with defaults.
func NewClaudeClient() *ClaudeClient {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	model := os.Getenv("CLAUDE_MODEL")
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	return &ClaudeClient{
		APIKey:    apiKey,
		Model:     model,
		MaxTokens: 4096,
		HTTPClient: &http.Client{
			Timeout: 3 * time.Minute,
		},
	}
}

// Message is a conversation message for the Claude API.
type Message struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// claudeRequest is the Messages API request body.
type claudeRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
}

// claudeResponse is the Messages API response body.
type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Chat sends a conversation to Claude and returns the assistant's text response.
func (c *ClaudeClient) Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error) {
	reqBody := claudeRequest{
		Model:     c.Model,
		MaxTokens: c.MaxTokens,
		System:    systemPrompt,
		Messages:  messages,
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeAPIURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("api call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claude API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("empty response from claude")
	}

	return claudeResp.Content[0].Text, nil
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./cmd/llmagent/...
```

(Will fail until other files exist — that's OK at this stage. Just check `claude.go` syntax.)

- [ ] **Step 3: Commit**

```bash
git add cmd/llmagent/claude.go
git commit -m "feat: add Claude Messages API client for LLM agents"
```

---

### Task 2: Role Definitions

**Files:**
- Create: `cmd/llmagent/roles.go`

- [ ] **Step 1: Define all 4 roles with system prompts and AgentCard metadata**

```go
package main

// Role defines an LLM agent role.
type Role struct {
	ID           string
	Name         string
	Description  string
	SkillID      string
	SkillName    string
	SystemPrompt string
}

var roles = map[string]Role{
	"legal": {
		ID:          "legal",
		Name:        "Legal Review Agent",
		Description: "Analyzes contracts for legal risks, compliance issues, and liability exposure",
		SkillID:     "contract-review",
		SkillName:   "Contract Risk Analysis",
		SystemPrompt: `You are an enterprise legal counsel specializing in contract risk assessment.

Analyze the provided deal for legal risks. Focus on:
- Limitation of liability and indemnification clauses
- Intellectual property ownership and licensing terms
- Termination conditions and exit clauses
- Regulatory compliance requirements
- Data protection and privacy obligations

Return your analysis as JSON (no markdown, no code fences, just raw JSON):
{
  "risk_level": "LOW" | "MEDIUM" | "HIGH" | "CRITICAL",
  "issues": [{ "clause": "...", "risk": "...", "recommendation": "..." }],
  "summary": "2-3 sentence overall assessment"
}`,
	},
	"finance": {
		ID:          "finance",
		Name:        "Finance Review Agent",
		Description: "Evaluates deal economics, pricing, margins, and discount compliance",
		SkillID:     "financial-assessment",
		SkillName:   "Financial Deal Assessment",
		SystemPrompt: `You are a corporate finance analyst specializing in deal economics.

Evaluate the financial viability of the provided deal. Analyze:
- Revenue and margin projections
- Pricing relative to standard rates
- Discount justification and policy compliance
- Payment terms and cash flow impact
- Revenue recognition implications

IMPORTANT: If the discount exceeds 20%, you MUST include a "needs_input" field requesting approval.

Return your analysis as JSON (no markdown, no code fences, just raw JSON):
{
  "margin_pct": <number>,
  "pricing_assessment": "...",
  "discount_analysis": { "discount_pct": <number>, "within_policy": <boolean>, "justification": "..." },
  "payment_terms": "...",
  "recommendation": "...",
  "needs_input": { "message": "...", "options": ["Approve", "Reject"] }
}
Only include needs_input if the discount exceeds 20%.`,
	},
	"technical": {
		ID:          "technical",
		Name:        "Technical Review Agent",
		Description: "Assesses implementation feasibility, technical risks, and resource requirements",
		SkillID:     "technical-feasibility",
		SkillName:   "Technical Feasibility Review",
		SystemPrompt: `You are a technical architect evaluating implementation feasibility.

Assess the technical viability of delivering the described deal. Evaluate:
- Technology stack compatibility and integration complexity
- Security and compliance requirements
- Scalability and performance considerations
- Team capability and resource requirements
- Implementation timeline and milestones

Return your analysis as JSON (no markdown, no code fences, just raw JSON):
{
  "feasibility": "HIGH" | "MEDIUM" | "LOW",
  "risks": [{ "area": "...", "severity": "...", "mitigation": "..." }],
  "resource_estimate": { "engineers": <number>, "months": <number> },
  "timeline": "...",
  "recommendation": "..."
}`,
	},
	"deal-review": {
		ID:          "deal-review",
		Name:        "Deal Review Agent",
		Description: "Synthesizes legal, financial, and technical analyses into Go/No-Go recommendation",
		SkillID:     "deal-synthesis",
		SkillName:   "Deal Go/No-Go Synthesis",
		SystemPrompt: `You are the chair of the deal review committee. You synthesize analyses from Legal, Finance, and Technical teams into a final Go/No-Go recommendation.

You will receive structured data from three analysts in the message. Weigh all factors:
- Legal risk severity and mitigation feasibility
- Financial viability and margin acceptability
- Technical implementation risk and timeline

Return your decision as JSON (no markdown, no code fences, just raw JSON):
{
  "decision": "GO" | "NO-GO" | "CONDITIONAL",
  "confidence": <number 0-1>,
  "rationale": "3-5 sentence explanation",
  "conditions": ["condition 1", "..."],
  "risk_summary": "1-2 sentence overall risk posture"
}
Include conditions only for CONDITIONAL decisions.`,
	},
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/llmagent/roles.go
git commit -m "feat: define 4 Deal Review agent roles with system prompts"
```

---

### Task 3: Agent Executor (A2A AgentExecutor Implementation)

**Files:**
- Create: `cmd/llmagent/executor.go`

- [ ] **Step 1: Implement the AgentExecutor interface**

This file bridges A2A protocol and Claude API. When a `SendMessage` arrives, it:
1. Extracts instruction and upstream data from the message parts
2. Builds conversation messages (system prompt + user message)
3. Calls Claude API
4. Parses JSON response
5. For finance role: checks `needs_input` field → returns `input-required` state
6. Returns completed task with artifact

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
)

// LLMExecutor handles A2A messages by calling Claude API.
type LLMExecutor struct {
	role   Role
	claude *ClaudeClient

	// Conversation history per context (contextId → messages)
	mu       sync.Mutex
	convos   map[string][]Message

	// For input-required: task waiting for input
	waitMu   sync.Mutex
	waitChs  map[string]chan string // taskId → channel for user input
}

// NewLLMExecutor creates an executor for the given role.
func NewLLMExecutor(role Role, claude *ClaudeClient) *LLMExecutor {
	return &LLMExecutor{
		role:    role,
		claude:  claude,
		convos:  make(map[string][]Message),
		waitChs: make(map[string]chan string),
	}
}

// HandleMessage processes an incoming A2A message.
// Returns the response text, whether input is required, and any error.
func (e *LLMExecutor) HandleMessage(ctx context.Context, contextID string, taskID string, text string, data map[string]any) (responseText string, inputRequired bool, inputMessage string, err error) {
	e.mu.Lock()
	convo := e.convos[contextID]

	// Build user message content
	userContent := text
	if data != nil {
		dataJSON, _ := json.MarshalIndent(data, "", "  ")
		userContent = text + "\n\nUpstream analysis data:\n" + string(dataJSON)
	}

	convo = append(convo, Message{Role: "user", Content: userContent})
	e.convos[contextID] = convo
	e.mu.Unlock()

	// Call Claude
	response, err := e.claude.Chat(ctx, e.role.SystemPrompt, convo)
	if err != nil {
		return "", false, "", fmt.Errorf("claude call: %w", err)
	}

	// Store assistant response in conversation history
	e.mu.Lock()
	e.convos[contextID] = append(e.convos[contextID], Message{Role: "assistant", Content: response})
	e.mu.Unlock()

	// For finance role: check if LLM output contains needs_input
	if e.role.ID == "finance" {
		// Try to parse as JSON and check for needs_input field
		var parsed map[string]any
		// Strip markdown code fences if present
		cleaned := strings.TrimSpace(response)
		if strings.HasPrefix(cleaned, "```") {
			lines := strings.Split(cleaned, "\n")
			if len(lines) > 2 {
				cleaned = strings.Join(lines[1:len(lines)-1], "\n")
			}
		}
		if err := json.Unmarshal([]byte(cleaned), &parsed); err == nil {
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

// HandleFollowUp processes a follow-up message (after input-required).
func (e *LLMExecutor) HandleFollowUp(ctx context.Context, contextID string, text string) (string, error) {
	e.mu.Lock()
	convo := e.convos[contextID]
	convo = append(convo, Message{Role: "user", Content: text})
	e.convos[contextID] = convo
	e.mu.Unlock()

	response, err := e.claude.Chat(ctx, e.role.SystemPrompt, convo)
	if err != nil {
		return "", err
	}

	e.mu.Lock()
	e.convos[contextID] = append(e.convos[contextID], Message{Role: "assistant", Content: response})
	e.mu.Unlock()

	return response, nil
}
```

Note: The exact integration with `a2a-go/v2` SDK's `AgentExecutor` interface (which returns `iter.Seq2[a2a.Event, error]`) must be adapted during implementation. This file provides the core logic; `main.go` will bridge it to the SDK interface.

- [ ] **Step 2: Commit**

```bash
git add cmd/llmagent/executor.go
git commit -m "feat: implement LLM agent executor with Claude API integration

Handles A2A messages, manages conversation history per context,
detects needs_input in finance agent output for human-in-the-loop."
```

---

### Task 4: Main Entry Point + A2A Server

**Files:**
- Create: `cmd/llmagent/main.go`

- [ ] **Step 1: Create the main server**

This file:
1. Parses `--role` and `--port` flags
2. Creates the AgentCard for the selected role
3. Sets up the A2A server using `a2a-go/v2` SDK
4. Serves the AgentCard at `/.well-known/agent-card.json`
5. Handles `SendMessage` via the LLMExecutor

```go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	roleFlag := flag.String("role", "", "agent role: legal, finance, technical, deal-review")
	port := flag.Int("port", 9091, "listen port")
	flag.Parse()

	if *roleFlag == "" {
		fmt.Fprintf(os.Stderr, "Usage: llmagent --role=<legal|finance|technical|deal-review> [--port=9091]\n")
		os.Exit(1)
	}

	role, ok := roles[*roleFlag]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown role: %s\nAvailable: legal, finance, technical, deal-review\n", *roleFlag)
		os.Exit(1)
	}

	claude := NewClaudeClient()
	if claude.APIKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required")
	}

	executor := NewLLMExecutor(role, claude)

	// Build AgentCard JSON
	card := buildAgentCard(role, *port)
	cardJSON, _ := json.MarshalIndent(card, "", "  ")

	// Set up HTTP server
	// Note: In actual implementation, use a2asrv.NewJSONRPCHandler() or
	// a2asrv.NewRESTHandler() from the SDK. The handler below is a
	// simplified version showing the routing logic.
	mux := http.NewServeMux()

	// AgentCard discovery endpoint
	mux.HandleFunc("GET /.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cardJSON)
	})

	// Health endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// A2A JSON-RPC endpoint — adapt to use a2asrv from SDK
	// The SDK's handler will route SendMessage, GetTask, CancelTask etc.
	// For now, register a placeholder that the SDK handler replaces.
	// See a2a-go/v2 documentation for exact server setup.

	log.Printf("%s listening on :%d (role: %s)", role.Name, *port, role.ID)

	addr := fmt.Sprintf(":%d", *port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func buildAgentCard(role Role, port int) map[string]any {
	return map[string]any{
		"name":        role.Name,
		"description": role.Description,
		"version":     "1.0.0",
		"url":         fmt.Sprintf("http://localhost:%d", port),
		"capabilities": map[string]any{
			"streaming":              false,
			"pushNotifications":      false,
			"stateTransitionHistory": true,
		},
		"skills": []map[string]any{{
			"id":          role.SkillID,
			"name":        role.SkillName,
			"description": role.Description,
		}},
		"defaultInputModes":  []string{"text/plain", "application/json"},
		"defaultOutputModes": []string{"application/json"},
	}
}
```

Important: The actual A2A server setup using `a2asrv` from the SDK will need to be adapted during implementation. Consult the SDK's examples for how to wire `AgentExecutor` → `a2asrv.Handler` → HTTP server.

- [ ] **Step 2: Test locally**

```bash
ANTHROPIC_API_KEY=<key> go run ./cmd/llmagent --role=legal --port=9091
```

In another terminal:
```bash
curl http://localhost:9091/.well-known/agent-card.json
```

Expected: Returns AgentCard for "Legal Review Agent".

- [ ] **Step 3: Commit**

```bash
git add cmd/llmagent/main.go
git commit -m "feat: add LLM agent main entry point with A2A server

Single binary with --role flag. Serves AgentCard at well-known URL.
Routes SendMessage to LLMExecutor."
```

---

### Task 5: Dockerfile

**Files:**
- Create: `cmd/llmagent/Dockerfile`

- [ ] **Step 1: Create multi-stage Dockerfile**

```dockerfile
# Build stage
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /llmagent ./cmd/llmagent

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /llmagent /llmagent
ENTRYPOINT ["/llmagent"]
```

- [ ] **Step 2: Test Docker build**

```bash
docker build -f cmd/llmagent/Dockerfile -t taskhub-llmagent .
docker run --rm -e ANTHROPIC_API_KEY=test taskhub-llmagent --role=legal --port=9091 &
```

Expected: Container starts, logs "Legal Review Agent listening on :9091".

- [ ] **Step 3: Commit**

```bash
git add cmd/llmagent/Dockerfile
git commit -m "feat: add Dockerfile for LLM agent container"
```

---

### Task 6: Docker Compose

**Files:**
- Create: `docker-compose.agents.yml`

- [ ] **Step 1: Create Docker Compose file**

```yaml
# docker-compose.agents.yml
# Deal Review agents — 4 LLM-powered A2A agents
#
# Usage:
#   docker compose -f docker-compose.agents.yml up --build
#
# Requires: ANTHROPIC_API_KEY in environment or .env file

services:
  legal-agent:
    build:
      context: .
      dockerfile: cmd/llmagent/Dockerfile
    command: ["--role=legal", "--port=9091"]
    ports:
      - "9091:9091"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    restart: unless-stopped

  finance-agent:
    build:
      context: .
      dockerfile: cmd/llmagent/Dockerfile
    command: ["--role=finance", "--port=9092"]
    ports:
      - "9092:9092"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    restart: unless-stopped

  technical-agent:
    build:
      context: .
      dockerfile: cmd/llmagent/Dockerfile
    command: ["--role=technical", "--port=9093"]
    ports:
      - "9093:9093"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    restart: unless-stopped

  deal-review-agent:
    build:
      context: .
      dockerfile: cmd/llmagent/Dockerfile
    command: ["--role=deal-review", "--port=9094"]
    ports:
      - "9094:9094"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    restart: unless-stopped
```

- [ ] **Step 2: Test Docker Compose**

```bash
docker compose -f docker-compose.agents.yml up --build -d
```

Verify all 4 agents are running:
```bash
curl http://localhost:9091/.well-known/agent-card.json  # Legal
curl http://localhost:9092/.well-known/agent-card.json  # Finance
curl http://localhost:9093/.well-known/agent-card.json  # Technical
curl http://localhost:9094/.well-known/agent-card.json  # Deal Review
```

- [ ] **Step 3: Commit**

```bash
git add docker-compose.agents.yml
git commit -m "feat: add Docker Compose for Deal Review agents

4 LLM agent containers: legal (9091), finance (9092),
technical (9093), deal-review (9094)."
```

---

### Task 7: End-to-End Deal Review Test

- [ ] **Step 1: Start all services**

```bash
# Terminal 1: Database reset + backend
make db-reset && make dev-backend

# Terminal 2: Agents
docker compose -f docker-compose.agents.yml up --build

# Terminal 3: Frontend
make dev-frontend
```

- [ ] **Step 2: Register all 4 agents**

Via frontend (http://localhost:3000/agents/register) or API:
```bash
for port in 9091 9092 9093 9094; do
  curl -X POST http://localhost:8080/api/orgs/local-org/agents \
    -H 'Content-Type: application/json' \
    -d "{\"url\": \"http://localhost:$port\"}"
done
```

Verify all 4 agents appear in the agent list.

- [ ] **Step 3: Create Deal Review task**

Via frontend or API:
```bash
curl -X POST http://localhost:8080/api/orgs/local-org/tasks \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "Deal Review: Acme Corp",
    "description": "Review the following deal for approval:\n\nClient: Acme Corporation\nDeal Value: $2,000,000 annual enterprise license\nDiscount: 35% off list price\nTerm: 3-year commitment\nScope: Full platform deployment with custom integrations\nPayment: Net-60 quarterly\nSpecial Terms: Unlimited users, dedicated support, SLA 99.9%"
  }'
```

- [ ] **Step 4: Verify DAG execution**

Expected flow:
1. Orchestrator creates DAG: Legal, Finance, Technical (parallel) → Deal Review
2. Legal, Finance, Technical agents receive instructions and call Claude
3. Finance agent detects 35% discount > 20% → returns `input-required`
4. TaskHub displays in chat: "[Finance Agent] Discount of 35% exceeds 20% policy..."
5. User types `@Finance Approved, CEO signed off` in chat
6. Finance agent completes with updated assessment
7. Deal Review agent receives all 3 outputs → returns Go/No-Go recommendation
8. Task completes with final recommendation in chat

- [ ] **Step 5: Verify frontend display**

Check:
- DAG view shows 4 nodes with correct status transitions
- Chat shows all agent messages including structured JSON output
- Finance agent's input request is visible
- Final Deal Review recommendation is displayed

- [ ] **Step 6: Run quality gate**

```bash
make check
```

- [ ] **Step 7: Commit any fixes**

```bash
git add -A
git commit -m "test: verify Deal Review scenario end-to-end

4 LLM agents successfully orchestrate deal review with human-in-the-loop
for discount approval."
```
