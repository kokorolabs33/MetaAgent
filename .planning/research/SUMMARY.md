# Project Research Summary

**Project:** TaskHub v2.0 — Wow Moment Milestone
**Domain:** A2A multi-agent platform — agent tool use, artifact rendering, streaming output, inbound webhooks
**Researched:** 2026-04-06
**Confidence:** HIGH

## Executive Summary

TaskHub v2.0 transforms the platform from a "agents chatting" demo into a "agents doing real work" demo. The milestone has four features: agent tool use (function calling via OpenAI SDK), rich artifact rendering (structured output as typed cards), streaming agent output (token-by-token delivery to the browser), and inbound webhooks (external events trigger tasks). Research across all four areas confirms these are well-understood patterns with clear implementation paths on the existing Go + Next.js + PostgreSQL stack. The single new backend dependency is the official OpenAI Go SDK (`github.com/openai/openai-go/v3@v3.30.0`); the frontend gains `remark-gfm` and `rehype-highlight` to replace the hand-rolled markdown renderer. Everything else — HMAC verification, streaming relay, artifact storage — uses existing Go stdlib and current infrastructure.

The recommended approach is dependency-driven: build tool use first (it produces the structured data everything else depends on), then artifact rendering (makes tool output legible), then streaming (the "wow" interaction that watches a tool-using agent think in real time), then inbound webhooks (fully independent, adds an external trigger surface). Each phase has clean boundaries. Tool use lives entirely inside `cmd/openaiagent/` and `internal/llm/`. Artifacts are a frontend rendering concern with no protocol changes needed. Streaming coordinates across the A2A client, executor, and broker using new event types through existing infrastructure. Webhooks are a new independent handler. The existing broker, store handler, SSE infrastructure, orchestrator, policy engine, and auth layer are all unchanged.

The primary risks are architectural, not library-selection risks. The three most dangerous failure modes are: (1) conversation history corruption when tool calls are added to the existing `ChatMessage` struct — this requires a struct redesign before any tool work proceeds; (2) token streaming events overwhelming the broker's 64-event channel buffer — this requires a separate ephemeral streaming channel that bypasses the event store entirely; and (3) prompt injection through webhook payloads — this requires HMAC verification plus content sanitization from day one. Each has a clear, confirmed prevention strategy from direct codebase analysis.

---

## Key Findings

### Recommended Stack

The existing stack (Go 1.26, Next.js 16, PostgreSQL, pgx, shadcn/ui, Zustand, SSE broker) requires no structural changes. Notably, `react-markdown@10.1.0` is already installed but never imported — the "upgrade" is activating it with plugin support, not adding a new dependency. The minimum additions are one backend package and two to three frontend packages.

**Core technologies added:**
- `github.com/openai/openai-go/v3` (v3.30.0): Official OpenAI Go SDK — replaces the 123-line hand-rolled HTTP client in `internal/llm/openai.go`. Required for function calling (tool definitions, tool call loop, argument accumulation) and streaming. The SDK's `ChatCompletionAccumulator` handles streaming plus tool call delta accumulation, which is the hardest part to implement correctly from scratch. Released 2026-03-25, requires Go 1.22+ (we have 1.26.1). HIGH confidence.
- `remark-gfm` (^4.0.1): GitHub Flavored Markdown plugin for react-markdown — enables table, task list, and strikethrough rendering in agent output. Verified compatible with react-markdown 10.x. HIGH confidence.
- `rehype-highlight` (^7.0.2): Syntax highlighting via lowlight/highlight.js — ~50KB bundle, 37 languages, integrates directly with react-markdown's rehype chain. Preferred over react-syntax-highlighter (deprecated, slow, security issues) and shiki (695KB+ bundle, WASM complexity). MEDIUM confidence for Next.js 16 App Router integration — needs build-time testing.
- `highlight.js` (^11, conditional): CSS theme import may require explicit install if the build cannot resolve the import from rehype-highlight's transitive dependency.

**Explicitly rejected:** `langchain-go` or any agent framework (TaskHub is the framework), `gorilla/websocket` (SSE is simpler and working), `@tanstack/react-query` (would require migrating all existing data-fetching), OpenAI Responses API (Chat Completions is sufficient and avoids a second API style), `shiki`/`react-shiki` (bundle too large), `svix/svix-webhooks` (stdlib HMAC is 15 lines).

### Expected Features

The current gap is precise: agents produce text via single-shot chat with no tool use, no structured artifacts, no streaming, and no external triggers. The "agents doing real work" claim requires closing all four gaps.

**Must have (table stakes for a 2026 A2A demo):**
- Agent tool use with web search — every multi-agent demo in 2026 shows agents retrieving live data; static-knowledge agents feel like chatbots
- Tool execution visibility in real-time — showing "Searching for: Go error handling..." while the agent works is non-negotiable UX
- Token-by-token streaming output — waiting 10-30 seconds for full responses feels broken
- Typed artifact cards (search results, code blocks, tables) — structured output rendered as rich UI, not raw text blobs
- Inbound webhook endpoint with HMAC verification — external events trigger tasks (GitHub PR, Slack slash command)

**Should have (differentiators for GitHub stars):**
- Tool call trace in the DAG view — sub-nodes within agent nodes showing the full reasoning chain; no open-source platform visualizes this
- Live search results preview as clickable source cards — user sees the same sources the agent sees before it reasons about them
- Webhook template library (GitHub + Slack presets) — one-click setup demonstrating breadth

**Defer to later phases:**
- Artifact diff view (requires versioned artifact storage not yet built)
- Cross-agent artifact passing (high complexity, changes orchestrator prompt construction)
- Streaming cost ticker (polish, not core demo value)
- Agent tool catalog UI (metadata page; build after tools are stable)
- MCP server support (muddies the A2A showcase story)
- A2UI protocol integration (nascent, Google-specific, large scope)
- File upload/binary artifact hosting (S3/MinIO complexity, out of scope)

**Critical path note:** Tool use must come first. It is the foundation for both streaming (streaming a tool-using agent is the compelling demo) and artifacts (tools produce the structured data worth rendering as cards). Inbound webhooks are independent and can be built in parallel or last.

### Architecture Approach

The four features are layered, not tangled. Each has clean integration boundaries with the existing system. The existing broker, event store, SSE stream handler, orchestrator, policy engine, and auth layer require zero changes.

**Major new components:**
1. **LLM tool client** (`internal/llm/openai.go` + `internal/llm/tools.go`) — SDK-based client with `ChatWithTools()` and `ChatStreaming()` methods; tool definition types, web search implementation, tool execution dispatcher
2. **Agent tool execution** (`cmd/openaiagent/`) — tool registry per agent role, tool call loop, `tasks/sendSubscribe` SSE handler for streaming
3. **A2A streaming layer** (`internal/a2a/streaming.go`) — SSE reader translating `TaskStatusUpdateEvent` / `TaskArtifactUpdateEvent` chunks into broker events; executor checks `AgentCard.capabilities.streaming` to choose polling vs streaming path
4. **Artifact rendering** (`web/components/artifacts/`) — `ArtifactCard` dispatcher + `SearchResultsCard`, `CodeArtifact`, `TableArtifact` components; artifact type schema shared between Go and TypeScript
5. **Inbound webhook ingestion** (`internal/handlers/webhook_ingestion.go`) — new handler outside auth middleware, HMAC verification, provider-specific parsers (GitHub, Slack, generic), idempotency via delivery ID

**Key patterns:**
- Streaming token deltas flow through broker as ephemeral events (not persisted); only the final assembled message is stored
- Artifact metadata lives in the existing `messages.metadata` JSONB column — no schema changes needed
- Inbound webhooks ack immediately (202), process in background goroutine — avoids duplicate task creation from provider retries
- Tool execution is internal to the agent binary; A2A protocol sees only final results

### Critical Pitfalls

1. **Tool call conversation history corruption** — The existing `llm.ChatMessage{Role, Content}` struct is incompatible with function calling. Tool call responses require a `ToolCalls []ToolCall` array on assistant messages (no `Content`), and tool results require `ToolCallID` linking back to the specific call. Building the tool loop on the existing struct causes silent conversation corruption and hallucinated results that are worse than clean failures. **Prevention:** Redesign `ChatMessage` before writing any tool loop code; add integration tests for the full tool round-trip. HIGH confidence from OpenAI community forums.

2. **Streaming token delta accumulation failure** — When streaming is active and the model calls a tool, `tool_calls[i].function.arguments` arrives split across 10-50+ SSE chunks. Naive parsing of each chunk as complete JSON crashes. Failing to key the accumulator by `tool_calls[index]` mixes arguments from parallel calls. **Prevention:** Use the SDK's `ChatCompletionAccumulator` which handles this natively. If using raw streaming, maintain a `map[int]*ToolCallAccumulator`. Only parse accumulated JSON after `finish_reason == "tool_calls"`. HIGH confidence, most-reported streaming + function calling bug.

3. **SSE broker buffer overflow under token streaming** — The existing broker uses 64-event buffered channels and silently drops events when full (direct codebase observation: `broker.go` line 25, line 61-65). Token streaming produces hundreds of events per subtask. Dropped token deltas are unrecoverable — they are not persisted to DB. **Prevention:** Do not route token deltas through the existing event broker. Use a separate ephemeral streaming channel. Persist only the final complete message as a standard `message` event. HIGH confidence from direct code analysis.

4. **Prompt injection via webhook payloads** — Unsanitized webhook payload content flows to the LLM orchestrator. An attacker controls what the Master Agent plans and what sub-agents execute, including tool calls (OWASP LLM01:2025). **Prevention:** Sanitize all webhook-derived text (strip control characters, 500-char title limit, 5000-char description limit), wrap in `[EXTERNAL CONTENT START/END]` delimiters in the orchestrator prompt, require HMAC verification from day one. HIGH confidence.

5. **Artifact type explosion without a schema contract** — Without a defined artifact schema, the frontend accumulates an unmaintainable switch statement checking heuristic patterns. The existing `a2a.MessagePart` struct uses `Data any` — completely untyped. Malformed artifacts crash the renderer. **Prevention:** Define a finite set of typed artifact schemas (`table`, `code`, `markdown`, `search_results`, `key_value`, `error`) with a `type` discriminator before building any rendering UI. Store schemas in `internal/a2a/artifacts.go` and `web/lib/artifact-types.ts`. HIGH confidence, most common mistake in multi-agent systems with heterogeneous output.

---

## Implications for Roadmap

Based on research, the dependency graph drives a four-phase structure. Tool use unlocks all other features; artifact rendering makes tool output legible; streaming creates the "wow" interaction; webhooks add external trigger capability independently.

### Phase 1: Agent Tool Use

**Rationale:** Foundation for all other v2.0 features. Tool use transforms agents from static-knowledge chatbots into agents retrieving real data. Every other feature is more impressive when agents produce real tool-based output. Has no dependencies on other v2.0 features and unblocks Phases 2 and 3.

**Delivers:** Web search capability in all agent roles via Tavily API; real-time tool call visibility in chat feed (`ToolCallCard` component); complete tool call conversation history (valid multi-turn tool loops); tool execution events via SSE (`tool_call_started`, `tool_call_completed`); OpenAI Go SDK integration replacing hand-rolled HTTP client.

**Addresses:** Table stakes features: agent tool use with web search, tool execution visibility.

**Avoids:**
- Pitfall 1 (conversation history corruption) — redesign `ChatMessage` struct as the very first task before any tool loop code
- Pitfall 10 (parallel tool call races) — default `parallel_tool_calls: false`; collect all results before updating conversation history
- Pitfall 7 (tool timeout cascading through DAG) — per-tool 15-second timeouts with graceful failure at the agent level, not the executor
- Pitfall 12 (OpenAI strict schema restrictions) — all tool schema properties must be required, `additionalProperties: false`

**Suggested sub-phases:**
1. Redesign `llm.ChatMessage` struct + integrate OpenAI Go SDK (`go get github.com/openai/openai-go/v3@v3.30.0`)
2. Implement web search tool (`internal/tools/websearch.go` with Tavily API) + tool execution dispatch in agent binary
3. Tool visibility events: SSE `tool_call_started` / `tool_call_completed` + `ToolCallCard.tsx` frontend component

**Research flag:** Well-documented patterns. OpenAI function calling is extensively documented. The SDK handles the hardest parts. The `ChatMessage` struct redesign is clearly specified in PITFALLS.md. No deeper research phase needed.

---

### Phase 2: Artifact Rendering

**Rationale:** Tool use produces structured data (search results, analysis tables, code). Without rich rendering, this data appears as raw JSON text and the "wow" is lost. Artifact rendering is predominantly frontend work, independent of streaming, and can proceed immediately after Phase 1 backend work completes. The schema definition sub-phase must come before any UI work.

**Delivers:** Typed artifact schema contract (`internal/a2a/artifacts.go` + `web/lib/artifact-types.ts`); `ArtifactCard` dispatcher component; `SearchResultsCard`, `CodeArtifact`, `TableArtifact` renderers; upgraded markdown rendering via `react-markdown` + `remark-gfm` + `rehype-highlight` (replacing the 240-line hand-rolled renderer in `ChatMessage.tsx`); copy/download on artifact cards.

**Addresses:** Table stakes: artifact rendering. Differentiator: live search results preview as clickable source cards.

**Avoids:**
- Pitfall 5 (artifact type explosion) — define schema contract as the first task of this phase before any UI is built
- Pitfall 13 (XSS via agent output) — use react-markdown defaults, no raw HTML rendering, explicit block-list for `script`, `iframe`, `object` elements
- Pitfall 14 (LLM abstraction leak) — keep artifact schema generic in shared types; OpenAI wire format stays inside `internal/llm/openai.go`

**Suggested sub-phases:**
1. Define artifact schema contract (shared Go + TypeScript types with `type` discriminator)
2. Agent prompt engineering for structured output + executor artifact metadata extraction (attach to `message.metadata`)
3. Frontend artifact card components + react-markdown upgrade

**Research flag:** The artifact type contract is a design decision that blocks all implementation and needs agreement between backend and frontend before either side builds. This is the phase kickoff decision, not ongoing research.

---

### Phase 3: Streaming Agent Output

**Rationale:** The highest-impact UX feature and the most architecturally complex. Streaming is most compelling when agents use tools — watching an agent search the web and type results token-by-token is the core "wow moment." Requires tool use (Phase 1) to be meaningful and benefits from artifact rendering (Phase 2) being in place so streamed artifacts render correctly as they arrive. The two-layer SSE architecture (OpenAI SSE → A2A SSE → broker relay → browser) must be designed carefully before implementation starts.

**Delivers:** Token-by-token streaming from LLM through A2A `tasks/sendSubscribe` to frontend SSE; `StreamingMessage.tsx` with blinking cursor animation; ephemeral `agent.streaming_chunk` events through broker (non-persisted, bypass event store); final message persisted as normal `message` event; `X-Accel-Buffering: no` header on SSE endpoints for proxy compatibility; poll fallback maintained for non-streaming agents.

**Addresses:** Table stakes: streaming agent output.

**Avoids:**
- Pitfall 2 (tool call delta accumulation failure) — use SDK's `ChatCompletionAccumulator`; index-keyed accumulator map for raw streaming; validate JSON only after `finish_reason`
- Pitfall 3 (broker buffer overflow) — separate ephemeral streaming channel, not the existing event broker
- Pitfall 6 (poll loop incompatibility) — check `AgentCard.capabilities.streaming`; keep poll path as fallback for non-streaming agents
- Pitfall 8 (SSE reconnection gap) — store partial accumulated text in Zustand keyed by subtask ID; reconnection resumes appending, does not restart
- Pitfall 11 (proxy buffering breaks streaming) — `X-Accel-Buffering: no` response header on all SSE endpoints; document Nginx config requirements

**Suggested sub-phases:**
1. Agent binary: `tasks/sendSubscribe` JSON-RPC handler + OpenAI SDK streaming (`stream.Next()` / `stream.Current()`)
2. A2A streaming client (`internal/a2a/streaming.go`) + executor streaming path in `runSubtask`
3. Frontend: `StreamingMessage` component + store handler for `agent.streaming_chunk` events + SSE reconnection recovery

**Research flag:** This phase needs a design session before implementation. The two-layer SSE coordination (A2A streaming client + broker relay), the non-persisted event strategy, and the tool call delta accumulation under streaming all require explicit technical design. Do not start coding without documenting the streaming pipeline design.

---

### Phase 4: Inbound Webhooks

**Rationale:** Fully independent of the other three features — no shared infrastructure dependencies. Placed last because it adds breadth (external triggers) rather than deepening the core agent interaction demo. The implementation pattern is well-understood and mirrors existing outbound webhook code. Security requirements are clear from research and must be addressed from day one, not deferred within the phase.

**Delivers:** `POST /api/webhooks/ingest/{source}` endpoint outside auth middleware; HMAC signature verification mirroring existing `internal/webhook/sender.go` signing; GitHub and Slack payload parsers; idempotency via delivery ID deduplication (`webhook_deliveries` table with unique constraint); `webhook_triggers` DB table with dual-secret support (`current_secret`, `previous_secret`); inbound webhook management UI; ack-first pattern (202 before processing).

**Addresses:** Table stakes: inbound webhook endpoint. Differentiator: webhook template library (GitHub + Slack presets).

**Avoids:**
- Pitfall 4 (prompt injection via webhook payloads) — HMAC verification required before any processing; content sanitization (length limits, control character stripping); `[EXTERNAL CONTENT]` delimiters in orchestrator prompt
- Pitfall 9 (duplicate tasks from provider retries) — ack-first (return 202 immediately on validation), background goroutine for processing, idempotency key from `X-GitHub-Delivery` or equivalent
- Pitfall 15 (secret rotation downtime) — dual-secret schema (`previous_secret` column) from day one even if rotation UI is deferred

**Suggested sub-phases:**
1. Backend: handler, ingestion logic, provider parsers (GitHub, Slack, generic), DB migration with dual-secret schema, idempotency table
2. Frontend: webhook trigger management UI + two template presets

**Research flag:** Standard patterns. The outbound webhook code in `internal/webhook/sender.go` is the direct reference implementation. No deeper research phase needed.

---

### Phase Ordering Rationale

- Tool use before everything: produces the structured data that makes all other features meaningful; has no dependencies on other v2.0 features
- Artifact rendering before streaming: streamed artifacts should render correctly on arrival; building the renderer first means the streaming phase does not also need to build display components under time pressure
- Streaming third: highest complexity; benefits from tools being stable and rendering being in place; the "wow" is watching a tool-using agent type results
- Webhooks last: fully independent; adding an external trigger surface is breadth, not depth; security requirements are clear

### Research Flags

Phases needing focused design before implementation:

- **Phase 3 (Streaming):** Requires a technical design session before coding. The two-layer SSE architecture, broker bypass for ephemeral events, and tool call delta accumulation under streaming all need explicit documentation before any of the three sub-phases begins.
- **Phase 2 (Artifact Schema):** The artifact type contract is a design decision that blocks all implementation on both backend and frontend. The schema definition must be the first deliverable of the phase.

Phases with standard, well-documented patterns (skip research-phase):

- **Phase 1 (Tool Use):** OpenAI function calling is extensively documented. The SDK handles the hard parts. `ChatMessage` struct redesign is clearly specified.
- **Phase 4 (Webhooks):** Standard REST + HMAC pattern with existing outbound code as direct reference.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Official SDK version verified (v3.30.0, 2026-03-25); compatibility matrix confirmed; remark-gfm 4.0.1 stable; rehype-highlight 7.0.2 verified. One medium-confidence note: rehype-highlight + Next.js 16 App Router integration needs build-time testing before committing. |
| Features | HIGH | Table stakes features are well-established 2026 patterns. Dependency order confirmed by multiple sources. Anti-features clearly scoped. Tavily vs Brave recommendation is medium confidence (vendor benchmarks); tool interface abstracts the provider so switching is low-cost. |
| Architecture | HIGH for tool use, artifacts, webhooks. MEDIUM for streaming. | Tool use, artifacts, and webhooks have clean integration paths grounded in direct codebase analysis. Streaming is the most complex integration — A2A streaming spec is well-defined, but implementing two-layer SSE coordination in Go introduces coordination risk. |
| Pitfalls | HIGH | Top 5 pitfalls all confirmed by direct codebase analysis (broker buffer size 64, ChatMessage struct, EventSource reconnection, existing HMAC in sender.go, untyped MessagePart.Data) and community-documented OpenAI function calling bugs. |

**Overall confidence:** HIGH

### Gaps to Address

- **Tavily vs Brave search API:** Research recommends Tavily for agent-optimized responses on the free tier (1000 searches/month). Brave has higher benchmark scores (14.89 vs 13.67) but returns raw results requiring extraction. Validate Tavily response quality in the context of TaskHub's agent prompts during Phase 1. The tool interface abstracts the provider — switching later is low-cost.
- **rehype-highlight + Next.js 16 App Router integration:** Flagged MEDIUM confidence in STACK.md. Test the CSS import (`highlight.js/styles/github-dark.css`) from the transitive dependency during Phase 2 setup; may need explicit `highlight.js` install.
- **OpenAI Responses API vs Chat Completions API:** FEATURES.md recommends Responses API; ARCHITECTURE.md recommends staying with Chat Completions to avoid a second API style. Resolved: use Chat Completions. The existing `internal/llm/openai.go` uses it, the SDK supports it fully, and function calling is complete in Chat Completions.
- **Streaming reconnection UX:** The Zustand partial-text accumulation approach for SSE reconnection is the right pattern, but the exact reconnection UX (placeholder vs. silent resume) is a product decision to make during Phase 3 planning.
- **Dual-secret webhook rotation UX:** The schema should include `previous_secret` from day one (Phase 4). The rotation workflow (when does the old secret expire, how does the user rotate) needs design before the management UI is built.

---

## Sources

### Primary (HIGH confidence)
- [openai/openai-go GitHub](https://github.com/openai/openai-go) — v3.30.0 release verified 2026-03-25; function calling and streaming examples verified
- [A2A Protocol Specification](https://a2a-protocol.org/latest/specification/) — `tasks/sendSubscribe`, `TaskStatusUpdateEvent`, `TaskArtifactUpdateEvent`, artifact Part types
- [A2A Streaming and Async](https://a2a-protocol.org/latest/topics/streaming-and-async/) — SSE event format, streaming task state machine
- [OpenAI Streaming Events Reference](https://developers.openai.com/api/reference/resources/chat/subresources/completions/streaming-events) — delta format, finish_reason values, tool call accumulation
- Direct codebase analysis — `internal/events/broker.go` (buffer 64, silent drop at line 61-65), `internal/llm/openai.go` (ChatMessage struct), `internal/executor/executor.go` (pollUntilTerminal at line 750), `internal/webhook/sender.go` (HMAC signing pattern), `web/lib/sse.ts` (EventSource reconnection), `internal/a2a/client.go` (MessagePart Data any)
- [remark-gfm npm](https://www.npmjs.com/package/remark-gfm) — v4.0.1 stable, react-markdown 10.x compatible
- [rehype-highlight npm](https://www.npmjs.com/package/rehype-highlight) — v7.0.2 verified

### Secondary (MEDIUM confidence)
- [OpenAI Function Calling Guide](https://platform.openai.com/docs/guides/function-calling) — tool schema requirements, strict mode, parallel_tool_calls
- [OpenAI Conversation State Guide](https://platform.openai.com/docs/guides/conversation-state) — tool message ordering requirements
- [Agentic Search Benchmark 2026](https://aimultiple.com/agentic-search) — Tavily vs Brave score comparison
- [Webhook security best practices](https://hookdeck.com/webhooks/guides/webhook-security-vulnerabilities-guide) — HMAC, replay prevention, idempotency
- [Webhook replay prevention](https://webhooks.fyi/security/replay-prevention) — timestamp window + idempotency store pattern
- [SSE behind Nginx](https://medium.com/@dsherwin/surviving-sse-behind-nginx-proxy-manager-npm-a-real-world-deep-dive-69c5a6e8b8e5) — proxy_buffering, X-Accel-Buffering header
- [Slack Webhook Triggers](https://api.slack.com/automation/triggers/webhook) — official Slack signing scheme
- [OpenAI Community: Streaming + Function Calls](https://community.openai.com/t/help-for-function-calls-with-streaming/627170) — real-world developer struggles with tool call streaming

### Tertiary (LOW confidence)
- [Multi-Agent Framework Comparison 2026](https://gurusup.com/blog/best-multi-agent-frameworks-2026) — context for "agents doing real work" positioning
- [Vercel ai-elements shiki issue #14](https://github.com/vercel/ai-elements/issues/14) — performance evidence against react-syntax-highlighter (community report, not independently verified)

---

*Research completed: 2026-04-06*
*Ready for roadmap: yes*
