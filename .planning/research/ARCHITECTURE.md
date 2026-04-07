# Architecture: v2.0 Feature Integration

**Domain:** A2A multi-agent platform -- agent tool use, artifact rendering, streaming output, inbound webhooks
**Researched:** 2026-04-06
**Scope:** How new features integrate with existing TaskHub architecture

## Executive Summary

The four v2.0 features touch every major layer of the existing system, but the integration surface is well-defined. Agent tool use lives entirely inside `cmd/openaiagent/` and `internal/llm/`. Artifact rendering is a frontend-only concern (new components in `web/components/chat/`). Streaming output requires coordinated changes across the A2A client, executor, broker, SSE handler, and frontend. Inbound webhooks are a new independent handler registered in `cmd/server/main.go`.

The key architectural insight: these features are layered, not tangled. Tool use changes the agent binary. Artifacts change how output is rendered. Streaming changes how output is delivered. Webhooks add a new entry point. They can be built in dependency order with clean boundaries.

---

## Feature 1: Agent Tool Use (OpenAI Function Calling)

### What Changes

Agent tool use adds OpenAI function calling to `cmd/openaiagent/`. Currently, agents make a single `ChatWithHistory` call and return the result text. With tool use, the agent must:

1. Define tools in the chat completions request
2. Detect `tool_calls` in the response
3. Execute tool functions (e.g., web search)
4. Send tool results back as `role: "tool"` messages
5. Get the final response and return it

### Components Modified

| Component | Change | Scope |
|-----------|--------|-------|
| `internal/llm/openai.go` | Add `ChatWithTools` method that handles tool definitions, `tool_calls` detection, and tool result loop | Medium |
| `cmd/openaiagent/openai.go` | Use `ChatWithTools` instead of `ChatWithHistory` when tools are configured | Small |
| `cmd/openaiagent/roles.go` | Add tool definitions per role (e.g., engineering gets web search) | Small |
| `cmd/openaiagent/main.go` | No structural change -- tool use is internal to the agent's LLM call | None |

### New Components

| Component | Purpose |
|-----------|---------|
| `internal/llm/tools.go` | Tool definition types, tool execution dispatcher, web search implementation |
| `cmd/openaiagent/tools.go` | Agent-specific tool registry, maps role to available tools |

### Data Flow Change

```
BEFORE:
  executor -> A2A SendMessage -> openaiagent -> LLM Chat -> text response -> A2A result

AFTER:
  executor -> A2A SendMessage -> openaiagent -> LLM ChatWithTools
    -> tool_calls detected -> execute tool (web search) -> send results back to LLM
    -> final text response with real data -> A2A result
```

### Integration Points

1. **LLM Client (`internal/llm/openai.go`)**: The existing `ChatWithHistory` sends a simple `chatRequest` with `model` and `messages`. The new `ChatWithTools` adds a `tools` field to the request and handles the multi-turn tool calling loop. The request format changes to:

```go
type chatRequest struct {
    Model    string        `json:"model"`
    Messages []ChatMessage `json:"messages"`
    Tools    []Tool        `json:"tools,omitempty"` // NEW
}

type Tool struct {
    Type     string       `json:"type"` // "function"
    Function ToolFunction `json:"function"`
}

type ToolFunction struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    Parameters  json.RawMessage `json:"parameters"`
    Strict      bool            `json:"strict,omitempty"`
}
```

The response format adds `tool_calls`:

```go
type chatResponse struct {
    Choices []struct {
        Message struct {
            Content   string     `json:"content"`
            ToolCalls []ToolCall `json:"tool_calls,omitempty"` // NEW
        } `json:"message"`
        FinishReason string `json:"finish_reason"` // NEW: "tool_calls" or "stop"
    } `json:"choices"`
}

type ToolCall struct {
    ID       string `json:"id"`
    Type     string `json:"type"`
    Function struct {
        Name      string `json:"name"`
        Arguments string `json:"arguments"` // JSON string
    } `json:"function"`
}
```

2. **Tool Result Messages**: After executing tools, results go back as:

```go
ChatMessage{
    Role:       "tool",
    Content:    resultJSON,
    ToolCallID: call.ID, // matches the tool_call.id
}
```

The `ChatMessage` struct needs a `ToolCallID` field. This is backward-compatible -- the field is omitted when empty.

3. **A2A Protocol**: No changes needed. Tool use is internal to the agent. The A2A response still returns text/artifact parts. The executor sees the same `SendResult` it always has.

4. **Audit Logging**: The executor already logs `agent.submit` and `agent.completed`. Tool use happens inside the agent binary, so tool call details should be logged there (stdout/stderr for now). Future: agent could return tool metadata in artifacts.

### Architecture Decision

Use the Chat Completions API directly (not the Responses API), because:
- The existing `internal/llm/openai.go` already uses Chat Completions
- Function calling is fully supported in Chat Completions
- No need to introduce a second API style
- The Responses API is newer but not necessary for our use case

**Confidence: HIGH** -- OpenAI function calling is well-documented and the integration is self-contained.

---

## Feature 2: Artifact Rich Rendering

### What Changes

Currently, `ChatMessage.tsx` renders all message content through `renderContent()` which handles markdown, JSON code blocks, and tables. Agent output from subtask completion arrives as a text string published via `publishMessage()` in the executor.

Artifact rendering adds structured output types that render as rich UI cards: tables, reports, code diffs, charts. This requires agents to output structured artifacts and the frontend to recognize and render them.

### Components Modified

| Component | Change | Scope |
|-----------|--------|-------|
| `web/components/chat/ChatMessage.tsx` | Detect artifact metadata in messages, delegate to artifact renderers | Medium |
| `web/lib/types.ts` | Add artifact type definitions | Small |
| `internal/a2a/protocol.go` | Add `type` field to `MessagePart` for typed artifacts | Small |
| `cmd/openaiagent/roles.go` | Update system prompts to instruct structured output format | Small |
| `internal/executor/executor.go` | Pass artifact type metadata through message publishing | Small |

### New Components

| Component | Purpose |
|-----------|---------|
| `web/components/artifacts/ArtifactCard.tsx` | Container component: artifact type detection, card chrome (title, expand, copy) |
| `web/components/artifacts/TableArtifact.tsx` | Renders tabular data as a styled table |
| `web/components/artifacts/CodeArtifact.tsx` | Renders code with syntax highlighting |
| `web/components/artifacts/ReportArtifact.tsx` | Renders structured report sections |
| `web/components/artifacts/JsonArtifact.tsx` | Renders JSON with collapsible tree view |

### Data Flow

The key question is: how do artifacts flow from agent to frontend?

**Current flow:**
```
Agent completes -> executor receives result.Artifacts (json.RawMessage)
  -> executor stores in subtask.output
  -> executor publishes artifact text as chat message (publishMessage)
  -> frontend renders text via ChatMessage.tsx
```

**New flow:**
```
Agent completes -> executor receives result.Artifacts with typed parts
  -> executor stores in subtask.output
  -> executor publishes message with artifact metadata
  -> SSE delivers message event with metadata.artifacts field
  -> ChatMessage.tsx detects artifacts, renders ArtifactCard components
```

### Artifact Format Convention

Rather than changing the A2A wire format, use the message `metadata` field (already JSONB in the database, already `metadata?: Record<string, unknown>` in TypeScript):

```typescript
interface ArtifactMetadata {
  artifacts?: Array<{
    type: "table" | "code" | "report" | "json" | "diff";
    title?: string;
    data: unknown; // type-specific payload
  }>;
}
```

Agents produce artifacts by formatting their output as JSON with a known schema. The executor parses the agent response, detects artifact format, and attaches metadata. The frontend reads `message.metadata.artifacts` and renders accordingly.

### Integration Points

1. **Agent System Prompts (`cmd/openaiagent/roles.go`)**: Prompts need explicit instructions to produce structured output in a known JSON format when the task calls for structured data (tables, reports). This is the simplest approach -- no protocol changes, just prompt engineering.

2. **Executor Message Publishing**: The `handleA2AResult` method currently calls `publishMessage(ctx, task.ID, agent.ID, agent.Name, outputStr)`. It needs to additionally parse the output for artifact markers and attach them as message metadata.

3. **Message Model**: The `messages` table already has a `metadata JSONB` column. The `Message` TypeScript interface already has `metadata?: Record<string, unknown>`. No schema changes needed.

4. **SSE**: The message event already includes all fields from the messages table including metadata. No SSE changes needed.

**Confidence: HIGH** -- Uses existing metadata fields, no protocol or schema changes.

---

## Feature 3: Streaming Agent Output

### What Changes

This is the most architecturally complex feature. Currently:

1. Executor sends A2A message to agent
2. Agent returns immediately with `state: "working"`
3. Executor polls with `tasks/get` every 2 seconds until terminal state
4. Final result arrives all at once

With streaming:

1. Executor sends A2A streaming request (`tasks/sendSubscribe`)
2. Agent responds with SSE stream
3. Tokens arrive incrementally as `TaskStatusUpdateEvent` and `TaskArtifactUpdateEvent`
4. Executor relays incremental tokens to the TaskHub SSE broker
5. Frontend renders tokens as they arrive

### Components Modified

| Component | Change | Scope |
|-----------|--------|-------|
| `internal/a2a/client.go` | Add `SendMessageStream` method that reads SSE responses | Large |
| `internal/a2a/protocol.go` | Add streaming event types (`TaskStatusUpdateEvent`, `TaskArtifactUpdateEvent`) | Medium |
| `internal/a2a/discovery.go` | Read `capabilities.streaming` from agent card | Small |
| `internal/executor/executor.go` | Use streaming client when agent supports it, relay tokens to broker | Large |
| `internal/events/broker.go` | No change -- already supports publishing arbitrary events | None |
| `internal/handlers/stream.go` | No change -- already streams events from broker | None |
| `web/lib/sse.ts` | No change -- already parses arbitrary event types | None |
| `web/lib/store.ts` | Add handler for new `agent.streaming_chunk` event type | Small |
| `web/lib/types.ts` | Add streaming-related type definitions | Small |
| `web/components/chat/ChatMessage.tsx` | Support rendering of in-progress streaming messages | Medium |
| `cmd/openaiagent/main.go` | Support `tasks/sendSubscribe` method, stream SSE responses | Large |
| `cmd/openaiagent/openai.go` | Add `ChatWithHistoryStream` that reads SSE chunks from OpenAI | Medium |

### New Components

| Component | Purpose |
|-----------|---------|
| `internal/a2a/streaming.go` | SSE reader, `TaskStatusUpdateEvent` / `TaskArtifactUpdateEvent` parsers |
| `web/components/chat/StreamingMessage.tsx` | Renders an in-progress message with cursor animation |

### Architecture: Two-Layer Streaming

The streaming architecture has two independent SSE layers:

```
Layer 1: Agent -> TaskHub (A2A streaming)
  OpenAI SSE -> openaiagent -> A2A SSE -> executor

Layer 2: TaskHub -> Browser (existing SSE)
  executor -> broker -> stream handler -> EventSource
```

**Layer 1 (A2A Streaming)**:

The A2A protocol defines `tasks/sendSubscribe` which returns `Content-Type: text/event-stream`. Each SSE event's `data` field is a JSON-RPC response containing either:
- `TaskStatusUpdateEvent`: status changes + intermediate messages
- `TaskArtifactUpdateEvent`: artifact chunks with `append` and `lastChunk` flags
- `Task`: final task state

The A2A client needs a `SendMessageStream` method that:
1. Sends the JSON-RPC request
2. Reads SSE events from the response body
3. Returns a channel of streaming events
4. Closes when the task reaches terminal state

```go
type StreamEvent struct {
    StatusUpdate   *TaskStatusUpdateEvent   // non-nil if status update
    ArtifactUpdate *TaskArtifactUpdateEvent  // non-nil if artifact chunk
    Task           *A2ATask                  // non-nil if final task state
}

func (c *Client) SendMessageStream(ctx context.Context, agentURL, contextID, taskID string, parts []MessagePart) (<-chan StreamEvent, error)
```

**Layer 2 (Browser Streaming)**:

The executor receives streaming events from the A2A client and relays them as broker events. The existing broker/SSE infrastructure handles delivery to the browser.

New event types published to broker:
- `agent.streaming_chunk`: token chunk from an agent (data: `{subtask_id, agent_id, content, is_final}`)
- `agent.streaming_complete`: stream finished

These events are NOT persisted to the event store (like `agent.status_changed`). They are ephemeral -- the final message IS persisted as a regular `message` event.

**Agent-Side Streaming**:

The `cmd/openaiagent/` binary needs to:
1. Handle `tasks/sendSubscribe` JSON-RPC method
2. Call OpenAI with `stream: true`
3. Read OpenAI SSE chunks
4. Write A2A SSE events back to the HTTP response

The OpenAI streaming format sends chunks as:
```
data: {"choices":[{"delta":{"content":"token"},"finish_reason":null}]}
```

The agent translates these into A2A streaming events:
```
data: {"jsonrpc":"2.0","id":"1","result":{"taskId":"...","status":{"state":"working","message":{"role":"agent","parts":[{"text":"token"}]}}}}
```

### Integration Points

1. **A2A Client (`internal/a2a/client.go`)**: The existing `SendMessage` method makes a synchronous HTTP call and reads the response body as JSON. The new `SendMessageStream` opens an SSE connection. The executor checks agent capabilities and chooses between `SendMessage` (poll) and `SendMessageStream` (stream).

2. **Executor DAG Loop**: The `runSubtask` method currently calls `SendMessage` then `pollUntilTerminal`. With streaming, it calls `SendMessageStream` and reads from the channel, publishing relay events to the broker. The final result still goes through `handleA2AResult`.

3. **Agent Capability Detection**: The agent's `AgentCard` includes `capabilities.streaming`. The executor's `loadAgents` query returns agent data that includes the stored agent card. Check `capabilities.streaming` to decide whether to use streaming.

4. **Broker Event Types**: The broker is generic -- it publishes `*models.Event` to channels. No broker changes needed. The new event types are just strings in the event's `Type` field.

5. **Frontend Store**: The `handleEvent` method in `store.ts` needs a new case for `agent.streaming_chunk` that either creates a new streaming message or appends to an existing one. A `StreamingMessage` component renders in-progress text with a blinking cursor.

6. **Non-Persisted Events**: Streaming chunks should NOT be saved to the events table (too many, too fast). Like `agent.status_changed`, they go through the broker only (ephemeral). The final complete message IS persisted.

### Fallback: Polling Still Works

Streaming is additive. If an agent does not support streaming (`capabilities.streaming: false`), the executor falls back to the existing poll-based flow. This means streaming can be shipped incrementally -- start with the agent-side, then add executor relay, then frontend rendering.

**Confidence: MEDIUM** -- Architecture is sound but this is the most complex integration. The A2A streaming spec is well-defined, but implementing SSE parsing in Go and coordinating two streaming layers introduces risk.

---

## Feature 4: Inbound Webhooks

### What Changes

Currently, TaskHub has *outbound* webhooks: the `webhook.Sender` fires HTTP requests when events occur. Inbound webhooks add a new entry point that receives external HTTP requests and creates tasks.

This is the most independent feature -- it adds a new handler and route with minimal coupling to existing code.

### Components Modified

| Component | Change | Scope |
|-----------|--------|-------|
| `cmd/server/main.go` | Register new webhook ingestion route | Small |
| `internal/db/migrations/` | New migration for `webhook_triggers` table | Small |

### New Components

| Component | Purpose |
|-----------|---------|
| `internal/handlers/webhook_ingestion.go` | Handler for `POST /api/webhooks/ingest/{source}` |
| `internal/webhook/ingestion.go` | Business logic: validate payload, map to task creation params |
| `internal/webhook/providers/` | Provider-specific parsers (Slack, GitHub, generic) |
| `web/app/webhooks/inbound/page.tsx` | UI for managing inbound webhook configurations |
| `web/components/webhook/InboundWebhookForm.tsx` | Form for creating/editing inbound webhook triggers |

### Data Flow

```
External Service (Slack, GitHub, etc.)
  -> POST /api/webhooks/ingest/{source}?token=xxx
  -> WebhookIngestionHandler
    -> Validate HMAC/token authentication
    -> Parse provider-specific payload
    -> Map to task title + description
    -> Create task via TaskHandler.Create logic (or direct DB insert + executor.Execute)
  -> Return 200 OK
```

### Database Schema

```sql
CREATE TABLE IF NOT EXISTS webhook_triggers (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    source      TEXT NOT NULL,             -- "slack", "github", "generic"
    secret      TEXT NOT NULL DEFAULT '',   -- HMAC validation secret
    token       TEXT NOT NULL DEFAULT '',   -- URL token for authentication
    config      JSONB NOT NULL DEFAULT '{}', -- provider-specific config
    is_active   BOOLEAN NOT NULL DEFAULT true,
    template_id TEXT,                       -- optional: use a workflow template
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Integration Points

1. **Route Registration**: The inbound webhook endpoint lives OUTSIDE the auth middleware group. External services cannot authenticate with session cookies. Instead, use per-webhook token or HMAC signature verification.

```go
// Public webhook ingestion (no auth middleware -- uses per-webhook token)
r.Post("/api/webhooks/ingest/{source}", webhookIngestionH.Ingest)
```

2. **Task Creation**: The ingestion handler creates a task the same way `TaskHandler.Create` does, then calls `executor.Execute`. It needs the same dependencies: DB pool, executor reference.

3. **Template Matching**: If a webhook trigger specifies a `template_id`, the created task references that template. The existing executor already handles `task.TemplateID`.

4. **Security**: Each inbound webhook has its own secret/token. Provider-specific validation:
   - **GitHub**: Validate `X-Hub-Signature-256` header with HMAC-SHA256
   - **Slack**: Validate `X-Slack-Signature` with signing secret
   - **Generic**: Validate `?token=` query parameter or `Authorization: Bearer` header

5. **Existing Outbound Webhooks**: The outbound webhook system (`webhook.Sender`) and inbound webhook system are completely independent. They share the `/api/webhooks` URL namespace but different sub-paths (`/api/webhooks` for config, `/api/webhooks/ingest/{source}` for ingestion).

**Confidence: HIGH** -- Simple, independent feature with well-understood patterns.

---

## Component Boundary Summary

### New Packages/Files

| Package/File | Layer | Depends On |
|-------------|-------|------------|
| `internal/llm/tools.go` | LLM | `net/http` (web search) |
| `internal/a2a/streaming.go` | A2A Protocol | `bufio`, `encoding/json` |
| `internal/webhook/ingestion.go` | Business Logic | DB, executor |
| `internal/webhook/providers/` | Business Logic | None (pure parsing) |
| `internal/handlers/webhook_ingestion.go` | HTTP | ingestion, DB |
| `cmd/openaiagent/tools.go` | Agent | `internal/llm` |
| `web/components/artifacts/*.tsx` | Frontend | React, types |
| `web/components/chat/StreamingMessage.tsx` | Frontend | React, types |

### Modified Packages (by impact)

| Package | Feature | Impact |
|---------|---------|--------|
| `internal/llm/openai.go` | Tool Use | Add `ChatWithTools`, `ChatWithHistoryStream` |
| `cmd/openaiagent/` | Tool Use + Streaming | Major: tool calling loop, SSE response handler |
| `internal/a2a/client.go` | Streaming | Add `SendMessageStream` |
| `internal/a2a/protocol.go` | Streaming + Artifacts | Add streaming event types, artifact type field |
| `internal/executor/executor.go` | Streaming | Add streaming path in `runSubtask` |
| `web/lib/store.ts` | Streaming + Artifacts | Handle new event types |
| `web/lib/types.ts` | All features | New type definitions |
| `web/components/chat/ChatMessage.tsx` | Artifacts | Artifact detection + delegation |
| `cmd/server/main.go` | Webhooks | Register new route |

### Unmodified Packages

These packages need zero changes:

| Package | Why Unchanged |
|---------|---------------|
| `internal/events/broker.go` | Generic pub/sub, already handles any event type |
| `internal/events/store.go` | Only persists events executor explicitly saves |
| `internal/handlers/stream.go` | Generic SSE writer, streams any broker events |
| `internal/orchestrator/` | Plans tasks, unaware of how agents execute |
| `internal/policy/` | Evaluates plans, unaware of agent internals |
| `internal/audit/` | Logs whatever the executor tells it to log |
| `internal/auth/` | Authentication layer, unaffected |
| `web/lib/sse.ts` | Generic SSE parser, handles any event type |
| `web/lib/api.ts` | May need new API functions for webhook management |

---

## Build Order (Dependency-Driven)

The features have this dependency graph:

```
Tool Use (independent)  ─────────────────┐
                                          ├──> All features complete
Artifact Rendering (independent) ────────┤
                                          │
Streaming Output ────────────────────────┤
  (depends on tool use for meaningful    │
   streaming demo, but not technically   │
   dependent)                            │
                                          │
Inbound Webhooks (independent) ──────────┘
```

**Recommended build order:**

1. **Agent Tool Use** -- Build first because it transforms agents from chat-only to actually useful. Every other feature is more impressive when agents produce real data.

2. **Artifact Rendering** -- Build second because tool use output (web search results, data analysis) needs rich rendering to look good. This is frontend-only after agents start producing structured output.

3. **Streaming Output** -- Build third because it is the most complex and benefits from tool use being done (watching an agent search the web and type results is the "wow moment"). Also, artifact rendering should be in place before streaming delivers artifacts incrementally.

4. **Inbound Webhooks** -- Build last because it is fully independent and adds a new entry point rather than enhancing the core flow. It is the least dependent on other features.

### Phase Breakdown Within Features

**Tool Use (3 sub-phases):**
1. `internal/llm/tools.go` -- tool types, web search implementation
2. `internal/llm/openai.go` -- `ChatWithTools` method
3. `cmd/openaiagent/` -- integrate tools into agent roles

**Artifact Rendering (3 sub-phases):**
1. Agent prompt engineering for structured output
2. Executor artifact metadata extraction + message metadata
3. Frontend artifact card components

**Streaming (4 sub-phases):**
1. `cmd/openaiagent/` -- SSE response for `tasks/sendSubscribe`, OpenAI streaming
2. `internal/a2a/streaming.go` -- SSE reader for A2A client
3. `internal/executor/` -- streaming path in `runSubtask`, relay to broker
4. Frontend -- `StreamingMessage` component, store handler

**Inbound Webhooks (2 sub-phases):**
1. Backend: handler, ingestion logic, provider parsers, migration
2. Frontend: webhook trigger management UI

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Coupling Tool Execution to A2A Protocol
**What:** Exposing tool calls as A2A protocol messages between executor and agent.
**Why bad:** Tool use is internal to the agent. The A2A protocol does not define tool call routing. Leaking tool internals across the protocol boundary creates tight coupling.
**Instead:** Tool execution stays inside the agent binary. The A2A response returns final results, optionally with tool metadata in artifact parts.

### Anti-Pattern 2: Persisting Streaming Tokens
**What:** Saving every streaming chunk to the events table.
**Why bad:** A single agent response can generate hundreds of chunks. This would bloat the events table and slow SSE replay on reconnection.
**Instead:** Streaming chunks go through the broker only (ephemeral). The final complete message is persisted as a single `message` event, same as today.

### Anti-Pattern 3: Modifying the Broker for Streaming
**What:** Adding special streaming-aware buffering or deduplication to the broker.
**Why bad:** The broker is deliberately simple (fan-out to channels). Adding streaming logic increases complexity and coupling.
**Instead:** The executor is responsible for the streaming relay logic. The broker just publishes events.

### Anti-Pattern 4: Single Webhook Endpoint for All Providers
**What:** One endpoint that inspects headers to determine the source.
**Why bad:** Different providers have different security validation, different payload formats, and different retry behaviors. A single endpoint becomes a complex router.
**Instead:** Use `POST /api/webhooks/ingest/{source}` where `{source}` routes to provider-specific validation and parsing.

---

## Scalability Considerations

| Concern | Current (100 users) | At 10K users | Notes |
|---------|---------------------|--------------|-------|
| Streaming memory | One goroutine per active subtask stream | Consider connection pooling | Each A2A stream holds an open HTTP connection |
| Streaming fan-out | Broker channels, 64-buffer | Buffer pressure under high fan-out | May need larger buffer for streaming events |
| Tool execution latency | Web search: 1-3s per call | Tool execution is per-agent, scales with agent count | No shared resource contention |
| Inbound webhook throughput | Direct task creation | Queue webhook processing | At scale, async processing prevents webhook timeouts |
| Artifact rendering | Client-side rendering | Still client-side | No server cost for artifact rendering |

---

## Sources

- [OpenAI Function Calling Guide](https://developers.openai.com/api/docs/guides/function-calling) -- Tool definition format, response handling
- [A2A Protocol Specification](https://a2a-protocol.org/latest/specification/) -- Protocol types, message format
- [A2A Streaming & Async](https://a2a-protocol.org/latest/topics/streaming-and-async/) -- `tasks/sendSubscribe`, SSE event format, `TaskStatusUpdateEvent`, `TaskArtifactUpdateEvent`
- [OpenAI Streaming Events](https://developers.openai.com/api/reference/resources/chat/subresources/completions/streaming-events) -- Delta format for streaming with tool_calls
- [trpc-a2a-go](https://pkg.go.dev/trpc.group/trpc-go/trpc-a2a-go/protocol) -- Go reference implementation of A2A streaming types
- Existing codebase: `internal/a2a/`, `internal/executor/`, `internal/llm/`, `cmd/openaiagent/`, `web/lib/store.ts`
