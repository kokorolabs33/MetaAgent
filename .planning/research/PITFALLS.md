# Domain Pitfalls

**Domain:** A2A meta-agent platform — agent tool use, artifact rendering, streaming output, inbound webhooks
**Project:** TaskHub
**Researched:** 2026-04-06
**Milestone scope:** v2.0 Wow Moment — function calling, rich artifacts, token streaming, webhook triggers

---

## Critical Pitfalls

Mistakes that cause rewrites, data corruption, or architectural dead-ends.

---

### Pitfall 1: Tool Call Conversation History Corruption

**What goes wrong:** The existing `internal/llm/openai.go` uses a simple `ChatMessage{Role, Content}` struct. OpenAI function calling requires three additional message types in the conversation history: (1) assistant messages with a `tool_calls` array instead of `content`, (2) `tool` role messages containing the result for each `tool_call_id`, and (3) the model's final response after processing tool results. If the conversation history omits any of these or gets the ordering wrong, the API returns a 400 error or the model hallucinates stale tool results.

**Why it happens:** The current `ChatMessage` struct only has `Role` and `Content` fields. Developers extend it by adding a `Content` field for tool results, forgetting that tool call messages have no `Content` -- they have a `ToolCalls` array. The `tool` role response also requires a `ToolCallID` field that links back to the specific call. This is a multi-field struct redesign, not a simple extension.

**Consequences:** Silent conversation corruption where the model "forgets" it called a tool, leading to infinite tool call loops or hallucinated results. In the A2A context, this means an agent appears to work but produces fabricated data, which is worse than a clean failure.

**Prevention:**
- Redesign `llm.ChatMessage` to support all OpenAI message types: add `ToolCalls []ToolCall`, `ToolCallID string`, and `Name string` fields. Use `json:"omitempty"` liberally so non-tool messages serialize cleanly.
- Write a conversation history validator that checks: every assistant message with `tool_calls` is followed by exactly one `tool` role message per call ID.
- Add integration tests that exercise the full tool call round-trip (send -> tool_calls finish_reason -> execute -> tool result -> final response).

**Detection:** Agent produces output that contradicts the tool's actual return value. API returns `400 Invalid request: messages with role 'tool' must be a response to a preceding message with 'tool_calls'`.

**Confidence:** HIGH -- OpenAI's API strictly enforces conversation history structure; this is the most common function calling bug reported in community forums.

**Phase relevance:** Must be addressed in the Tool Use phase, before any streaming or artifact work depends on it.

---

### Pitfall 2: Streaming + Tool Calls Delta Accumulation Failure

**What goes wrong:** When streaming is enabled and the model decides to call a tool, the response does not arrive as a single JSON blob. Instead, tool call arguments arrive as string deltas across multiple SSE chunks -- the `tool_calls[i].function.arguments` field is split across 10-50+ chunks. Developers either (a) try to parse each delta as complete JSON and crash, or (b) forget to accumulate by tool call index, mixing arguments from parallel tool calls.

**Why it happens:** The existing `openai.go` client reads the full response body synchronously with `io.ReadAll`. Switching to streaming means handling `data: [DONE]`, `finish_reason: "tool_calls"` vs `finish_reason: "stop"`, and accumulating partial argument strings keyed by `tool_calls[index]`. This is fundamentally different from streaming plain text tokens.

**Consequences:** Tool calls with truncated or garbled JSON arguments. Since the arguments are passed to tool execution, this causes either tool execution errors or, worse, partial arguments that happen to be valid JSON but represent the wrong operation (e.g., a search query truncated to a different, valid query).

**Prevention:**
- Build the streaming client as a separate method (`ChatStream` or `ChatWithHistoryStream`) rather than modifying the existing synchronous path.
- Maintain a `map[int]*ToolCallAccumulator` indexed by `tool_calls[i].index`. Each accumulator concatenates argument deltas until `finish_reason == "tool_calls"`.
- Only parse the accumulated JSON after the stream completes or hits `finish_reason`.
- Always validate the accumulated JSON against the tool's schema before executing.

**Detection:** Tool execution fails with JSON parse errors. Or tools receive subtly wrong parameters that produce unexpected results.

**Confidence:** HIGH -- This is the single most-reported streaming + function calling integration bug in the OpenAI developer community.

**Phase relevance:** Critical if streaming and tool use are combined. If built in separate phases, the streaming phase must plan for tool call delta handling from day one, even if tools are disabled initially.

---

### Pitfall 3: SSE Broker Channel Buffer Overflow During Token Streaming

**What goes wrong:** The current `events.Broker` uses buffered channels with capacity 64 (`make(chan *models.Event, 64)`). The existing event types (task lifecycle, status changes) produce maybe 5-20 events per subtask. Token-level streaming could produce hundreds or thousands of events per subtask (one per token). When the frontend consumer lags (tab backgrounded, slow render), the channel fills and the broker's `Publish` method silently drops events via its `default` case in the select statement.

**Why it happens:** The broker was designed for coarse-grained lifecycle events, not fine-grained streaming. The drop-on-full design is correct for lifecycle events (the frontend catches up from DB), but token deltas are not persisted to the database -- they are ephemeral. A dropped token delta means a gap in the streamed text that can never be recovered.

**Consequences:** Users see garbled, incomplete text that jumps from one part of a sentence to another. The final "completed" message has the full text (since that comes from the DB), but the streaming UX -- the entire point of the feature -- is broken.

**Prevention:**
- Do NOT stream individual tokens through the existing event broker. Instead, create a separate streaming channel/mechanism for token deltas.
- Option A: Dedicated streaming endpoint per subtask that proxies directly from the OpenAI stream to the client SSE connection, bypassing the broker entirely.
- Option B: Batch tokens into chunks (e.g., 5-10 tokens per event or every 100ms) to reduce event volume by 10-20x while still feeling real-time.
- Option C: Increase buffer size for streaming-enabled subscriptions and use a ring buffer that overwrites old tokens (newer tokens are more useful than older ones).
- Persist the final complete text as a regular message event; use streaming only for the in-progress display.

**Detection:** Compare final rendered text against the completed message content. If they differ, tokens were dropped during streaming.

**Confidence:** HIGH -- direct analysis of the existing `broker.go` code at line 25 (buffer size 64) and lines 61-65 (silent drop behavior).

**Phase relevance:** Must be solved before shipping streaming. This is an architectural decision that affects the streaming endpoint design.

---

### Pitfall 4: Inbound Webhook SSRF and Payload Injection

**What goes wrong:** Inbound webhooks accept HTTP requests from external sources (GitHub, Slack, etc.) and use the payload to create tasks. If the webhook handler passes unsanitized payload data into the task title or description, which then flows to the LLM orchestrator, an attacker can inject prompt instructions via the webhook payload. Example: a GitHub issue title containing adversarial instructions becomes the task description sent to the Master Agent.

**Why it happens:** The webhook payload parsing extracts fields like `title`, `body`, `text` from the incoming JSON and maps them to task creation parameters. Without sanitization, the LLM treats attacker-controlled content as instructions.

**Consequences:** Prompt injection via webhook -- an attacker controls what the Master Agent plans and what sub-agents execute. In a tool-use context, this could mean agents performing web searches or other tool actions directed by the attacker.

**Prevention:**
- Sanitize all webhook-derived text: strip control characters, enforce length limits (e.g., 500 chars for title, 5000 for description), reject payloads with known injection patterns.
- Wrap webhook-sourced content in clear delimiters in the orchestrator prompt: `[EXTERNAL CONTENT START]...[EXTERNAL CONTENT END]` with instructions to treat it as data, not instructions.
- Rate-limit inbound webhook endpoints aggressively (e.g., 10 requests/minute per webhook config).
- Require HMAC signature verification for all inbound webhooks (the existing outbound webhook sender already has HMAC signing in `internal/webhook/sender.go` -- mirror this for inbound).
- Log all webhook-triggered task creations with the source webhook ID for audit trail.

**Detection:** Audit logs show task descriptions that contain instruction-like language not originating from a human user. Anomalous agent behavior on webhook-triggered tasks.

**Confidence:** HIGH -- prompt injection via webhooks is a well-documented attack vector in LLM-integrated systems (OWASP LLM01:2025).

**Phase relevance:** Must be addressed in the webhook phase. Security cannot be deferred.

---

### Pitfall 5: Artifact Type Explosion Without a Schema Contract

**What goes wrong:** Different agents produce different artifact types (tables, code diffs, reports, search results, charts). Without a defined artifact schema, the frontend ends up with a growing switch statement that checks for heuristic patterns (`if data has rows and columns, render as table`). Each new artifact type requires frontend changes, and malformed artifacts crash the renderer.

**Why it happens:** The A2A protocol defines `TextPart`, `DataPart`, and `FilePart` as generic containers but does not prescribe internal structure for `DataPart`. Teams add artifact types ad-hoc ("the search agent returns results as an array") without a shared schema registry. The existing `a2a.MessagePart` struct in `internal/a2a/client.go:81-84` uses `Data any` -- completely untyped.

**Consequences:** Frontend crashes on unexpected artifact shapes. Agents produce slightly different formats for the same concept (one uses `{rows, columns}`, another uses `{headers, data}`). Maintenance burden grows linearly with each new agent or tool type.

**Prevention:**
- Define a finite set of artifact types with explicit JSON schemas: `table`, `code`, `markdown`, `search_results`, `key_value`, `error`. Each has a `type` discriminator field.
- Enforce the schema at the agent level: the agent binary validates its output against the schema before returning it in the A2A artifact.
- Build a single `ArtifactRenderer` React component that dispatches on `type` with a fallback to raw JSON display for unknown types -- never crash, always degrade gracefully.
- Store artifact schemas in a shared location (e.g., `internal/a2a/artifacts.go` for Go, `web/lib/artifact-types.ts` for TypeScript) that both sides import.

**Detection:** Frontend console errors when rendering artifacts. Agents returning artifacts that render as raw JSON instead of rich cards.

**Confidence:** HIGH -- this is the most common mistake in multi-agent systems with heterogeneous output types.

**Phase relevance:** Must be defined before building artifact rendering UI. The schema is the contract between the agent tool-use phase and the artifact rendering phase.

---

## Moderate Pitfalls

---

### Pitfall 6: Poll Loop Incompatibility with Streaming A2A Responses

**What goes wrong:** The current executor uses `pollUntilTerminal()` with a 2-second interval to check agent status (`internal/executor/executor.go:750`). If agents switch to streaming responses (A2A `tasks/sendSubscribe`), the executor still polls, missing the real-time stream entirely. Worse, if a streaming agent returns `state: "working"` to the initial `tasks/send` and expects the client to subscribe for updates, the poll loop works but adds unnecessary latency (up to 2 seconds per token batch).

**Why it happens:** The executor was designed for request-response A2A interactions. Streaming A2A is a different transport (SSE from agent to executor) that requires the executor to act as an SSE client, not a poller.

**Prevention:**
- Keep the poll loop as the default for non-streaming agents (backward compatibility).
- Check the agent's `AgentCard.capabilities.streaming` flag before deciding the communication strategy.
- For streaming agents, implement an SSE client in the executor that subscribes to the agent's stream and forwards events through the platform's broker.
- Do not try to retrofit streaming into the poll loop -- they are fundamentally different patterns.

**Detection:** Streaming agents work but feel sluggish (2-second update intervals instead of real-time). Or streaming agents hang because their SSE stream is never consumed.

**Confidence:** MEDIUM -- depends on whether agents implement A2A streaming or continue using request-response with async polling.

**Phase relevance:** Can be deferred if agents use request-response initially. Must be addressed before claiming A2A streaming compliance.

---

### Pitfall 7: Tool Execution Timeout Cascading Through DAG

**What goes wrong:** A tool call (e.g., web search) hangs or takes 30+ seconds. The agent's A2A task stays in `working` state. The executor's `pollUntilTerminal` has a 5-minute timeout (`internal/executor/executor.go:756`), so it waits. Meanwhile, downstream subtasks that depend on this one are blocked. If multiple subtasks in the DAG hit slow tools, the entire task can take 10-15 minutes for what should be a 1-minute operation.

**Why it happens:** The current system treats all agent work as opaque -- it has no visibility into whether the agent is thinking, waiting for a tool, or stuck. Tool execution adds a new failure mode (external service timeouts) that the DAG executor was not designed for.

**Prevention:**
- Add per-tool execution timeouts (e.g., 15 seconds for web search) enforced at the agent level, not the executor level.
- Have agents report intermediate status via A2A messages: `"Searching the web..."`, `"Processing results..."` -- this gives the executor and user visibility.
- Add a subtask-level timeout that is shorter than the 5-minute poll timeout (e.g., 3 minutes for tool-enabled subtasks).
- Implement graceful tool failure: if a tool times out, the agent should return a completed response noting the tool failure rather than hanging indefinitely.

**Detection:** Task execution times spike dramatically after enabling tool use. Subtasks sit in `working` state for minutes.

**Confidence:** MEDIUM -- the severity depends on tool reliability. Web search tools are generally fast, but custom tools or chained tool calls can be slow.

**Phase relevance:** Address during the tool use phase. Build timeout handling into the agent binary from the start.

---

### Pitfall 8: Frontend SSE Reconnection Loses Streaming Context

**What goes wrong:** The existing SSE client (`web/lib/sse.ts`) relies on `EventSource` auto-reconnection with `Last-Event-ID`. For lifecycle events, this works because all events are persisted to the database and replayed on reconnection. Token streaming events are ephemeral (not stored in DB). When a reconnection happens mid-stream, the frontend gets the replay of lifecycle events but loses all streamed tokens, showing a blank or incomplete agent response until the final completed message arrives.

**Why it happens:** `EventSource` reconnects automatically but the server can only replay persisted events. Token deltas are not persisted (storing every token in PostgreSQL would be absurd for performance).

**Prevention:**
- On reconnection, query the current subtask state from the API. If a subtask is `working`, fetch its partial output (if the agent stores it) or show a "streaming in progress, reconnecting..." placeholder.
- Store partial accumulated text in a Zustand store keyed by subtask ID. On reconnection, resume appending to the existing text rather than starting fresh.
- Consider a separate dedicated SSE connection for streaming content that is independent of the lifecycle event stream. This avoids mixing ephemeral and persistent data on the same channel.

**Detection:** Users report seeing blank agent outputs that suddenly "snap" to the final text. Happens more frequently on spotty networks.

**Confidence:** MEDIUM -- direct analysis of `web/lib/sse.ts` shows EventSource auto-reconnection is the only recovery mechanism.

**Phase relevance:** Must be addressed in the streaming phase, specifically in the frontend implementation.

---

### Pitfall 9: Webhook Replay and Duplicate Task Creation

**What goes wrong:** External services (GitHub, Slack) retry webhook deliveries if they do not receive a 200 response within their timeout window (typically 5-10 seconds). If task creation takes longer than this (e.g., because the orchestrator LLM call takes 8 seconds), the webhook sender retries, and a second identical task is created.

**Why it happens:** The natural approach processes webhook payloads synchronously -- creates the task and waits for planning to begin before responding. The webhook sender times out and retries. Since there is no idempotency check, the retry creates a duplicate task.

**Prevention:**
- Ack immediately: Return 200/202 as soon as the webhook payload is validated and queued, before any task creation begins. Use a background goroutine or work queue for actual processing.
- Implement idempotency via a deduplication key: hash the webhook payload (or use the provider's delivery ID like `X-GitHub-Delivery`) and store it in a `webhook_deliveries` table with a TTL. Reject duplicates.
- Add a unique constraint or dedup check: `INSERT INTO webhook_deliveries (delivery_id) VALUES ($1) ON CONFLICT DO NOTHING` -- if the insert is a no-op, skip processing.

**Detection:** Duplicate tasks with identical titles created within seconds of each other. Webhook delivery logs on the provider side show retries.

**Confidence:** HIGH -- GitHub retries after 10 seconds, Slack after 3 seconds. This is a guaranteed issue if processing takes longer than the retry window.

**Phase relevance:** Must be handled in the webhook phase. Build the ack-first pattern from day one.

---

### Pitfall 10: Parallel Tool Calls Producing Race Conditions in Agent State

**What goes wrong:** OpenAI models can issue multiple tool calls in a single response (parallel tool calls). If the agent binary executes them concurrently and both modify shared state (e.g., both write to the conversation history, or both try to produce artifacts), the results can interleave or overwrite each other. The current `cmd/openaiagent/main.go` uses `sync.Mutex` on `taskState` for basic protection, but this does not cover the tool execution phase which happens outside the lock.

**Why it happens:** Developers enable parallel tool calls for performance without considering that tool execution may have side effects or ordering requirements. The OpenAI API defaults `parallel_tool_calls` to `true`.

**Prevention:**
- Start with `parallel_tool_calls: false` for safety. Enable parallel execution only after confirming tools are side-effect-free and order-independent.
- If enabling parallel calls, execute tools concurrently but collect all results before continuing the conversation -- do not interleave tool execution with conversation history updates.
- Use a dedicated mutex or channel for tool result collection to prevent interleaving.

**Detection:** Agents produce garbled output that mixes results from two different tool calls. Conversation history shows tool results in the wrong order.

**Confidence:** MEDIUM -- depends on whether parallel tool calls are enabled. HIGH if they are.

**Phase relevance:** Address during tool use implementation. Set `parallel_tool_calls: false` as the default.

---

## Minor Pitfalls

---

### Pitfall 11: Nginx/Proxy Buffering Silently Breaks Token Streaming

**What goes wrong:** If TaskHub is deployed behind a reverse proxy (Nginx, Cloudflare, AWS ALB), the proxy's default response buffering aggregates SSE events into large chunks. Token streaming appears to work in local dev but in production, users see text arrive in bursts every 5-10 seconds instead of token-by-token.

**Prevention:**
- Set `X-Accel-Buffering: no` response header on all SSE endpoints (add to `StreamHandler.Stream` and `StreamHandler.MultiStream` in `internal/handlers/stream.go`).
- Document the required Nginx config: `proxy_buffering off;`, `proxy_http_version 1.1;`, `proxy_read_timeout 600;`.
- Add a streaming health check endpoint that sends one event per second -- if deployment breaks streaming, this endpoint reveals it immediately.

**Confidence:** HIGH -- this is the most common SSE deployment issue.

**Phase relevance:** Address during streaming deployment, not during development (local dev works fine without this).

---

### Pitfall 12: OpenAI Structured Output `strict: true` Schema Restrictions

**What goes wrong:** When using `strict: true` for function parameters (recommended for reliable tool arguments), OpenAI enforces that all object properties must be `required`, `additionalProperties` must be `false`, and the schema must not use `default` values. Developers define schemas with optional fields and get cryptic 400 errors.

**Prevention:**
- Always make all properties required in tool schemas. Use sentinel values (empty string, -1) instead of omitting optional fields.
- Set `additionalProperties: false` on every nested object, not just the top level.
- Test tool schemas against the OpenAI schema validation endpoint before integrating.

**Confidence:** HIGH -- documented requirement that is frequently overlooked.

**Phase relevance:** Address during tool schema definition in the tool use phase.

---

### Pitfall 13: Artifact Rendering XSS via Markdown or HTML in Agent Output

**What goes wrong:** Agents return markdown or HTML content in artifacts. If the frontend renders this with an unsanitized markdown renderer, a compromised or prompt-injected agent can inject JavaScript into the UI.

**Prevention:**
- Use `react-markdown` (already in the project at `web/package.json`) which does not render raw HTML by default.
- Explicitly disable rendering of `script`, `iframe`, `object`, `embed` elements in any custom renderers.
- For code blocks in artifacts, use a syntax highlighter that escapes HTML entities.
- Never render agent-generated content as raw HTML.

**Confidence:** HIGH -- the project already uses react-markdown which is safe by default, but new artifact renderers must maintain this discipline.

**Phase relevance:** Address during artifact rendering implementation.

---

### Pitfall 14: LLM Client Abstraction Leaking OpenAI-Specific Concepts

**What goes wrong:** The tool calling implementation is built directly against OpenAI's `tool_calls` format. If TaskHub later needs to support other LLM providers (Anthropic tool_use, Google function_calling), every tool-related function is OpenAI-specific and must be rewritten.

**Prevention:**
- Define a provider-agnostic tool call interface in `internal/llm/` (e.g., `ToolCall{Name, Arguments}`, `ToolResult{CallID, Output}`).
- The OpenAI-specific serialization (delta accumulation, `tool_calls` array format) stays in an OpenAI adapter layer.
- The agent binary and executor interact with the generic interface.

**Confidence:** LOW -- this is a forward-looking concern. TaskHub currently only uses OpenAI, and premature abstraction can slow development. A clean separation of the OpenAI HTTP layer from tool execution logic is sufficient.

**Phase relevance:** Consider during tool use design, but do not over-engineer. Keep the OpenAI HTTP details contained in `internal/llm/openai.go`.

---

### Pitfall 15: Webhook Secret Rotation Causes Downtime

**What goes wrong:** When rotating webhook secrets (for security best practices), all previously configured webhooks using the old secret start failing HMAC verification. If there is no grace period where both old and new secrets are accepted, webhook-triggered tasks stop working during rotation.

**Prevention:**
- Support dual secrets: store both `current_secret` and `previous_secret` in the webhook config. Accept either during verification.
- Add a `secret_expires_at` field for the previous secret so it is automatically retired.
- Log secret rotation events in the audit trail.

**Confidence:** MEDIUM -- only relevant after webhooks are in production and secrets need rotation.

**Phase relevance:** Can be deferred to a follow-up, but design the schema with the `previous_secret` column from day one.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|---|---|---|
| Agent Tool Use | Conversation history corruption (P1) | Redesign ChatMessage struct before building tool loop |
| Agent Tool Use | Parallel tool call races (P10) | Default to `parallel_tool_calls: false` |
| Agent Tool Use | Tool timeout cascading (P7) | Per-tool timeouts with graceful failure at agent level |
| Agent Tool Use | OpenAI strict schema rules (P12) | Test schemas early, all fields required |
| Artifact Rendering | Type explosion without schema (P5) | Define artifact schema contract first, before any UI |
| Artifact Rendering | XSS via agent output (P13) | Use react-markdown defaults, no raw HTML rendering |
| Artifact Rendering | Abstraction leak (P14) | Keep OpenAI wire format in llm package, expose generic interface |
| Streaming Output | Broker buffer overflow (P3) | Separate streaming channel, do not reuse event broker |
| Streaming Output | Tool call delta accumulation (P2) | Build accumulator with index-keyed map, validate before execute |
| Streaming Output | SSE reconnection gap (P8) | Store partial text in Zustand, handle reconnect gracefully |
| Streaming Output | Proxy buffering (P11) | X-Accel-Buffering header, document deploy config |
| Streaming Output | Poll loop vs streaming (P6) | Check agent capabilities, support both patterns |
| Inbound Webhooks | Prompt injection via payload (P4) | Sanitize payloads, HMAC verification, content delimiters |
| Inbound Webhooks | Duplicate tasks from retries (P9) | Ack-first pattern with idempotency key |
| Inbound Webhooks | Secret rotation downtime (P15) | Dual-secret support in schema from day one |

---

## Sources

- [OpenAI Function Calling Guide](https://platform.openai.com/docs/guides/function-calling) -- tool schema requirements, parallel_tool_calls, strict mode
- [OpenAI Streaming Events Reference](https://developers.openai.com/api/reference/resources/chat/subresources/completions/streaming-events) -- delta accumulation format, finish_reason values
- [OpenAI Community: Streaming + Function Calls](https://community.openai.com/t/help-for-function-calls-with-streaming/627170) -- real-world developer struggles with tool call streaming
- [OpenAI Conversation State Guide](https://platform.openai.com/docs/guides/conversation-state) -- tool message ordering requirements
- [A2A Protocol Specification](https://a2a-protocol.org/latest/specification/) -- streaming operations, artifact format, Part types
- [Webhook Security Vulnerabilities (Hookdeck)](https://hookdeck.com/webhooks/guides/webhook-security-vulnerabilities-guide) -- HMAC, replay prevention, idempotency
- [Webhook Security Best Practices (Hooque)](https://hooque.io/guides/webhook-security/) -- dual secret rotation, timestamp windows
- [Webhook Replay Prevention (webhooks.fyi)](https://webhooks.fyi/security/replay-prevention) -- timestamp window + idempotency store
- [SSE Streaming LLM Guide (Pockit)](https://pockit.tools/blog/streaming-llm-responses-web-guide/) -- SSE infrastructure, buffering issues, Go implementation patterns
- [SSE Behind Nginx (Medium)](https://medium.com/@dsherwin/surviving-sse-behind-nginx-proxy-manager-npm-a-real-world-deep-dive-69c5a6e8b8e5) -- proxy_buffering, chunk boundary corruption
- [Nginx SSE Configuration (OneUptime)](https://oneuptime.com/blog/post/2025-12-16-server-sent-events-nginx/view) -- proxy_buffering off, X-Accel-Buffering header
- Direct codebase analysis: `internal/events/broker.go` (buffer size 64, silent drop), `internal/llm/openai.go` (ChatMessage struct), `internal/executor/executor.go` (pollUntilTerminal), `cmd/openaiagent/main.go` (taskState mutex scope), `web/lib/sse.ts` (EventSource reconnection), `internal/a2a/client.go` (MessagePart Data any), `internal/webhook/sender.go` (HMAC signing pattern)
