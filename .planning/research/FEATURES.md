# Feature Landscape

**Domain:** Multi-agent platform — agent tool use, artifact rendering, streaming output, inbound webhooks
**Researched:** 2026-04-06
**Milestone:** v2.0 Wow Moment
**Focus:** New features only (existing foundation assumed built and working)

---

## Context: What Already Exists

The following are built and working. This document does NOT re-categorize them:

- Master Agent decomposes tasks into DAG subtasks via LLM
- Team agents communicate via A2A protocol (HTTP polling + native)
- Real-time SSE streaming with DAG visualization
- Agent status indicators (online/working/idle/offline)
- Chat intervention (@mention agents mid-execution)
- Parallel multi-task dashboard with live status
- Templates, policies, analytics, audit pages
- OpenAI-powered task decomposition (gpt-4o-mini via hand-rolled HTTP client)
- Outbound webhook infrastructure (CRUD, HMAC signing, delivery)
- Custom markdown renderer in ChatMessage.tsx (headings, tables, lists, code fences, @mentions)

**The gap:** Agents currently only produce text via single-shot chat. No tool use, no structured artifacts, no streaming, no inbound event triggers. The demo feels like "agents chatting" rather than "agents working."

---

## Table Stakes

Features users of multi-agent platforms expect in 2026. Missing any of these makes the "agents doing real work" claim feel hollow.

### 1. Agent Tool Use (Function Calling)

| Feature | Why Expected | Complexity | Dependencies on Existing |
|---------|--------------|------------|--------------------------|
| Web search tool (first tool) | Every AI demo in 2026 shows agents retrieving live data. Without it, agents are stale-knowledge chatbots producing generic answers. | Medium | `internal/llm/openai.go` needs tool support; `cmd/openaiagent` needs tool execution loop |
| Tool call -> result -> continue loop | The standard agentic pattern: LLM calls tool, gets result, reasons about it, optionally calls more tools. Users of CrewAI, LangGraph, AutoGen all expect iterative tool use. | Medium | `internal/llm/openai.go` currently does single-shot `ChatWithHistory` only -- no `tool_calls` handling, no response iteration |
| Tool execution visibility | Users must see WHAT the agent is doing while it works. "Searching web for: Go error handling best practices..." is 2026 table stakes. Opaque "working..." is unacceptable. | Low | Existing SSE broker + event store can carry new `tool_call` event types. Frontend needs a `ToolCallCard` component. |
| Strict mode / schema validation | OpenAI best practice: `strict: true` on tool definitions ensures valid JSON args. Prevents hallucinated parameters that break tool execution. | Low | New code in llm client. Pure implementation, no design decisions. |
| Multiple parallel tool calls | OpenAI models can return multiple tool calls in a single response. Must handle all of them, not just the first. This is documented OpenAI best practice. | Low | Part of the tool loop implementation. Execute in parallel, collect results, send all back. |

**Key technical note:** The current `internal/llm/openai.go` is a 123-line hand-rolled HTTP client that talks to `api.openai.com/v1/chat/completions`. It has no tool support, no streaming, no response iteration. For tool use + streaming, replace it with the official `github.com/openai/openai-go/v3` SDK (v3.30.0, requires Go 1.22+). The SDK provides `responses.FunctionToolParam` for tool definitions, built-in streaming via `client.Responses.NewStreaming()`, and proper tool call loop support.

### 2. Artifact Rendering (Rich Structured Output)

| Feature | Why Expected | Complexity | Dependencies on Existing |
|---------|--------------|------------|--------------------------|
| Typed artifact parts in messages | A2A protocol defines artifacts with TextPart, DataPart, FilePart. When agents produce structured data (search results, analysis tables), it must be typed, not a text blob. | Medium | `a2a.Artifact` and `a2a.MessagePart` types exist in `internal/a2a/protocol.go` but only text parts are used. Need artifact model in DB + typed message metadata. |
| Search results as clickable cards | When web search returns results, render them as source cards with title, URL, snippet -- not raw JSON. Users see the same sources the agent sees. | Medium | New `SearchResultsCard.tsx` component. Tool result events need structured data format. |
| Table/data artifact as styled card | When an agent produces comparison data or analysis, render it as a formatted table card, not a code block. | Low-Med | Existing `renderMarkdown` handles tables but only from markdown syntax. Need `DataCard.tsx` for structured JSON -> table rendering. |
| Code block with syntax highlighting | Agents producing code diffs, config snippets, or analysis need proper syntax coloring. Current renderer shows code in plain `<pre>` tags. | Low | Add lightweight syntax highlighter. Shiki (Next.js native) or Prism (smaller). |
| Copy/download on artifact cards | Users expect to copy code blocks and download generated content with one click. | Low | UI-only addition to artifact card components. |

**Key technical note:** The current `Message` type has `content: string` and `metadata?: Record<string, unknown>`. The approach is additive: add `artifacts` to the Message model (array of typed parts matching A2A Artifact structure) and `artifact_type` to metadata for renderer dispatch. Existing text messages continue rendering via the markdown renderer unchanged.

### 3. Streaming Agent Output

| Feature | Why Expected | Complexity | Dependencies on Existing |
|---------|--------------|------------|--------------------------|
| Token-by-token streaming from LLM to frontend | Users expect to see agents "thinking" in real-time. The typing effect is standard UX in 2026. Waiting 10-30s for a full response feels broken. | High | Three-layer change: (1) OpenAI streaming API in Go SDK, (2) A2A `SendStreamingMessage` support in agent binary, (3) SSE relay through broker to frontend. |
| Streaming during tool use | When agent calls a tool, show "Searching..." status, then stream the reasoning response after tool results return. The two features interleave. | Medium | Depends on both tool use and streaming being implemented. Tool call events show pre-stream; response tokens show post-tool. |
| Progressive markdown rendering | Markdown arriving mid-stream must render without layout thrashing. Headings, lists, tables need to appear as they form, not flash/reflow. | Medium | Existing custom markdown renderer in `ChatMessage.tsx` is not streaming-aware. Options: (a) buffer + re-render on each chunk, (b) use `react-markdown` with stream adapter, (c) use Streamdown library (streaming-specific). |
| Per-agent streaming indicator | Show which agent is actively streaming vs. done. Visual distinction between "working" and "streaming response." | Low | `TypingIndicator.tsx` exists. Extend to show streaming state with token count or progress indicator. |

**Key technical note -- the three-layer streaming problem:**

1. **LLM Layer:** OpenAI streaming returns SSE events with delta chunks. The official Go SDK handles this with `stream.Next()` / `stream.Current()`.

2. **A2A Protocol Layer:** The A2A spec defines `SendStreamingMessage` which returns SSE events (`TaskStatusUpdateEvent`, `TaskArtifactUpdateEvent`). The `cmd/openaiagent` binary must serve this. The `CardCapability.Streaming` field is already in the codebase (set to `false`).

3. **Frontend Relay Layer:** The existing SSE broker publishes `models.Event` per task. For streaming token deltas, use a separate non-persisted event channel. The `stream_delta` event type gets relayed to the frontend via SSE but does NOT get stored in the event store (would overwhelm it). Only the final assembled message gets persisted.

### 4. Inbound Webhooks

| Feature | Why Expected | Complexity | Dependencies on Existing |
|---------|--------------|------------|--------------------------|
| POST endpoint that creates a task | External systems (GitHub, Slack, CI/CD) send a webhook; TaskHub creates and executes a task from the payload. This is the standard automation pattern (n8n, Zapier, Trigger.dev all do this). | Medium | Outbound webhook infrastructure exists (`internal/webhook/`, `internal/handlers/webhooks.go`, DB table `webhook_configs`). Need new INBOUND handler + API key/HMAC auth. |
| Webhook secret verification (HMAC) | Security table stakes. GitHub signs payloads with HMAC-SHA256; Slack signs with its own scheme. Must verify before processing. | Low | Existing outbound webhook sender already does HMAC signing (`X-TaskHub-Signature: sha256=...`). Adapt verification logic for inbound. |
| Source-specific payload parsing | GitHub sends different JSON than Slack sends. Need parsers that extract task title + description from source-specific payloads. At minimum: GitHub (PR/issue events), Slack (slash command), generic JSON. | Medium | New code: template-based mapping from webhook payload fields to task creation params. |
| Webhook management UI (inbound) | CRUD for inbound webhook endpoints with generated URLs, secret display, and event log. | Low | Outbound webhook settings page exists at `/settings/webhooks`. Extend or add parallel page for inbound. |

**Key technical note:** The pattern is to flip the existing outbound infrastructure. New DB table `inbound_webhook_configs` (id, name, secret, source_type, task_template, created_by, created_at). New endpoint `POST /api/hooks/:id` (public, no session auth -- verified by HMAC). Payload parser maps to task creation, then calls existing `TaskHandler.Create` logic internally.

---

## Differentiators

Features that would make TaskHub stand out from other multi-agent platforms. Not expected, but create the "wow moment" that gets GitHub stars.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Tool call trace in DAG view | Show tool calls as sub-nodes within agent nodes in the React Flow DAG. User sees the full reasoning chain: decompose -> agent starts -> searches web -> reasons -> produces artifact. No open-source platform visualizes the tool-use chain this way. | Medium | Extends existing React Flow DAG. Needs tool-call events persisted as timeline entries. Collapsible sub-nodes within agent nodes. |
| Live search results preview | When web search tool returns results, show them as clickable source cards in the chat BEFORE the agent reasons about them. User sees the same data the agent is processing. Builds trust and transparency. | Medium | New `SearchResultsCard.tsx` component. Tool result events carry structured search data. |
| Artifact diff view | When an agent revises a previous artifact (second draft, updated analysis), show a before/after diff. Unique to platforms with artifact versioning. | Medium | Requires artifact_id + version tracking. Uses existing React diff library or simple custom diffing. |
| Webhook template library | Pre-built configs for GitHub PR review, Slack slash command, Linear issue, Jira ticket. One-click setup with auto-filled payload mappings. | Low | Seed data + template selection UI. Great for demo -- shows breadth without complexity. |
| Agent tool catalog UI | Page showing which tools each agent has access to, with descriptions and usage stats. Shows the "tool economy" of the platform. | Low | Metadata display only. Agent card already has skills; extend with tool definitions. |
| Streaming cost ticker | Show estimated token cost accumulating in real-time as the agent streams its response. Makes cost transparency visceral. | Low | Audit logger already tracks tokens. Connect streaming events to a running cost counter in the UI. |
| Cross-agent artifact passing | Agent A produces a research report; Agent B's prompt includes it as structured context (not just the text blob in the channel). Artifacts flow through the A2A artifact mechanism, not just messages. | High | Changes orchestrator prompt construction and A2A message composition. Requires the artifact model to be solid first. |

---

## Anti-Features

Features to explicitly NOT build. Each adds complexity without advancing the "wow moment."

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Custom tool builder UI | A visual tool definition editor is a massive scope increase (schema builder, parameter validation UI, test runner). Diminishing returns for a demo platform where developers read code. | Hardcode 2-3 well-chosen tools (web search, code analysis). Let developers add tools by writing Go code. Document the pattern in README. |
| A2UI protocol integration | A2UI is interesting but nascent (Google-specific, launched mid-2026), adds a rendering dependency layer, and distracts from the A2A protocol focus. | Use simple typed artifact parts (text, table, code, search_results) with custom React renderers. Lighter, faster, and fully controlled. |
| File upload/storage | A2A's FilePart supports binary artifacts, but hosting files adds infra complexity (S3/MinIO, presigned URLs, size limits, virus scanning). | Support TextPart and DataPart only. File artifacts can reference external URLs without TaskHub hosting them. |
| Voice/audio streaming | A2A supports non-text modalities but this is entirely out of scope for a developer-focused text-based demo. | Text only. Clearly stated in scope. |
| Agent-to-agent tool delegation | Agent A asks Agent B to run a tool on its behalf. Adds inter-agent protocol complexity without clear demo value. | Each agent manages its own tools. Cross-agent collaboration happens through channel messages and artifacts, not tool sharing. |
| Generic MCP server support | MCP (Model Context Protocol) is a different protocol from A2A. Supporting both protocols muddies the "A2A showcase" story and doubles the protocol surface area. | Stay A2A-focused. If agents need external data, wrap it as an OpenAI function tool within the agent binary, not as an MCP server. |
| Webhook retry queue with dead letter | Enterprise-grade webhook reliability (exponential backoff, DLQ, retry dashboard, delivery guarantees) is overkill for a demo platform. | Simple fire-and-forget for outbound (existing pattern). For inbound, validate and process synchronously. Log failures to audit. |
| Real-time collaborative document editing | Multiple agents editing the same document simultaneously with OT/CRDT conflict resolution. | Sequential agent turns (existing pattern). Each agent reads latest channel state, produces new output. This is how A2A is designed to work. |
| Generative UI (agent produces arbitrary React components) | Exciting concept (CopilotKit, Vercel AI SDK do this) but requires a component sandbox, security boundary, and runtime compilation layer. Huge scope. | Pre-built artifact renderers for known types (table, code, search results). Agent output maps to these renderers via type metadata. |

---

## Feature Dependencies

```
                    Tool Use (Function Calling)
                   /            |             \
                  v             v              v
          Web Search      Tool Call       Tool Execution
             Tool         Visibility        Events (SSE)
                            |                  |
                            v                  v
                   Streaming Output     Tool Trace in DAG
                   /          \          (differentiator)
                  v            v
           Token Streaming   Streaming
           (LLM -> SSE)     Cost Ticker
                  |          (differentiator)
                  v
          Partial Markdown
            Rendering

          Artifact Rendering
           /        |         \
          v         v          v
     Artifact    Artifact    Artifact
      Model     Card UI     Copy/Download
    (DB + API)     |
          \        v
           \  Search Results Card
            \  (differentiator)
             \
              v
         Artifact Diff View
          (differentiator)

          Inbound Webhooks  (independent track)
           /           \
          v             v
     Webhook       Payload
     Endpoint      Parsers
     + Auth           |
          \          v
       Webhook Template Library
          (differentiator)
```

### Critical Path

**Tool Use must come first.** It is the foundation for both streaming (streaming a tool-using agent is the compelling demo) and artifacts (tools produce the structured data worth rendering as cards). Without tool use, streaming is just "watch text appear faster" and artifacts have nothing structured to render.

**Inbound webhooks are independent** and can be built in parallel with the tool use + streaming track.

### Dependency Matrix

| Feature A | Requires Feature B | Reason |
|-----------|-------------------|--------|
| Streaming agent output | Tool use (function calling) | Streaming a simple chat response is unimpressive. The demo value is streaming a tool-using agent: search -> reason -> produce artifact. |
| Artifact card rendering | Tool use (function calling) | Without tools, agents produce only text. With tools (web search results, computed data), agents produce structured data worth rendering. |
| Tool trace in DAG | Tool execution events (SSE) | Cannot visualize what isn't tracked. |
| Streaming cost ticker | Streaming + audit logger | Needs real-time token events to display running cost. |
| Artifact diff view | Artifact model (DB + API) | Needs versioned artifact storage before diffs make sense. |
| Webhook template library | Inbound webhook endpoint | Templates configure the endpoint, not vice versa. |
| Live search results preview | Web search tool + tool visibility events | Search results must flow as structured tool-result events. |
| Cross-agent artifact passing | Artifact model (DB + API) | Must store and reference artifacts before passing them between agents. |

---

## MVP Recommendation

### Must Build (Core "Wow Moment") -- ordered by dependency

1. **Upgrade LLM client to official OpenAI Go SDK** -- Foundation for everything. Replace `internal/llm/openai.go` with `github.com/openai/openai-go/v3`. Enables tool definitions, tool call loop, and streaming in one library.

2. **Web search tool for agents** -- THE feature that transforms the demo. Use Tavily API (free tier: 1000 searches/month, agent-optimized responses, ~1s latency). Define as an OpenAI function tool with `strict: true`. The engineering agent searches for code patterns, the marketing agent searches for market data, etc.

3. **Tool execution visibility via SSE** -- Show tool calls in real-time in the chat feed: "Searching: best practices for Go error handling..." appears while the agent works. Publish `tool_call_started` and `tool_call_completed` events through the existing SSE broker. New `ToolCallCard.tsx` component in frontend. Low complexity, high perceived value.

4. **Streaming agent output** -- Token-by-token streaming from LLM through A2A to frontend. Three-layer implementation: SDK streaming -> agent binary SSE -> platform SSE relay -> React progressive rendering. Use non-persisted `stream_delta` events to avoid overwhelming event store.

5. **Basic artifact cards** -- When agents produce structured data (search results, tables, code), render as styled cards. Start with 3 artifact types: `search_results`, `code`, `table`. Does not require full artifact DB storage initially -- use typed message metadata. Upgrade to full artifact model in a follow-up phase.

6. **Inbound webhook endpoint** -- `POST /api/hooks/:id` with HMAC verification. GitHub PR and Slack slash command templates for the demo. Flips existing outbound webhook pattern.

### Defer to Later Phases

| Feature | Why Defer | When |
|---------|-----------|------|
| Artifact diff view | Requires versioned artifact storage that doesn't exist yet | After artifact model is solid (Phase 2+) |
| Cross-agent artifact passing | High complexity, changes orchestrator prompt construction | Future milestone |
| Streaming cost ticker | Nice polish but not core demo value | Phase 3+ |
| Tool trace in DAG view | Requires significant React Flow sub-node work | Phase 2+ |
| Agent tool catalog UI | Metadata page; build after tools are working and stable | Phase 3+ |
| Webhook template library beyond 2 | Start with GitHub + Slack only; add more based on user demand | Phase 2+ |

---

## Existing Code Touchpoints

| New Feature | Files to Modify | Files to Create |
|-------------|----------------|-----------------|
| LLM SDK upgrade | `internal/llm/openai.go` (rewrite), `cmd/openaiagent/openai.go` (rewrite), `go.mod` (add openai-go/v3) | -- |
| Tool use | `cmd/openaiagent/main.go` (add tool loop + tool execution), `cmd/openaiagent/roles.go` (add tool definitions per role) | `internal/tools/websearch.go` (Tavily client), `internal/tools/tools.go` (tool registry interface) |
| Tool visibility | `internal/events/broker.go` (new event types), `web/lib/types.ts` (new SSE event types), `web/lib/store.ts` (handle tool events) | `web/components/chat/ToolCallCard.tsx` |
| Streaming (backend) | `internal/a2a/client.go` (add SendStreamingMessage), `internal/a2a/protocol.go` (streaming types), `internal/executor/executor.go` (streaming execution path), `cmd/openaiagent/main.go` (serve streaming endpoint) | `internal/a2a/streaming.go` (SSE parse/relay helpers) |
| Streaming (frontend) | `web/components/chat/GroupChat.tsx` (streaming state), `web/lib/sse.ts` (delta handling), `web/lib/store.ts` (streaming message state) | `web/components/chat/StreamingMessage.tsx` (progressive render) |
| Streaming relay | `internal/handlers/stream.go` (relay stream deltas without persisting) | -- |
| Artifact rendering | `web/components/chat/ChatMessage.tsx` (dispatch to renderers), `web/lib/types.ts` (artifact types), `internal/models/models.go` (artifact fields on Message) | `web/components/artifacts/SearchResultsCard.tsx`, `web/components/artifacts/CodeBlock.tsx`, `web/components/artifacts/DataCard.tsx` |
| Artifact storage | `internal/models/models.go` (Artifact struct) | `internal/db/migrations/NNN_artifacts.sql` |
| Inbound webhooks | `cmd/server/main.go` (register route) | `internal/handlers/inbound_hooks.go`, `internal/db/migrations/NNN_inbound_webhooks.sql`, `web/app/settings/inbound-webhooks/page.tsx` |

---

## Technical Decision Points

### Web Search API: Tavily vs Brave Search

| Criterion | Tavily | Brave Search |
|-----------|--------|-------------|
| Agent Score (2026 benchmark) | 13.67 | 14.89 (highest) |
| Latency | ~998ms | ~669ms (fastest) |
| Free tier | 1000 searches/month | 2000 searches/month |
| Response format | Pre-summarized for LLMs (ideal for function calling) | Raw search results (need extraction) |
| SDK availability | npm + Python (no official Go SDK) | REST API (no SDK needed) |
| Setup complexity | API key + POST request | API key + POST request |

**Recommendation: Tavily for v2.0.** Despite Brave's better benchmark scores, Tavily returns pre-formatted summaries optimized for LLM consumption. This means the search tool produces immediately useful context without a summarization step. The free tier is sufficient for demo usage. If scale demands it, migrate to Brave later -- the tool interface abstracts the provider.

### OpenAI API: Chat Completions vs Responses API

| Criterion | Chat Completions | Responses API |
|-----------|-----------------|---------------|
| Status | Maintained "indefinitely" | Primary recommended API |
| Tool support | `tools` + `tool_calls` in response | `ToolUnionParam` with function tools |
| Streaming | `stream: true` with SSE chunks | `NewStreaming()` with typed events |
| Go SDK support | Full | Full (v3.30.0) |
| Migration effort | Low (current code is Chat Completions) | Medium (different request/response types) |

**Recommendation: Responses API.** OpenAI recommends it for new integrations. Since we are rewriting the LLM client anyway (hand-rolled HTTP -> SDK), there is no incremental cost to targeting the Responses API. The Chat Completions API will be maintained but the Responses API gets all new features first.

### Streaming Architecture: Persist Everything vs Delta Relay

| Approach | Pros | Cons |
|----------|------|------|
| Persist every token delta | Full replay from DB; crash recovery | Thousands of rows per response; DB thrash; event store overwhelmed |
| Non-persisted delta relay | Fast; no DB load; clean event store | No replay of partial responses; need final-message persistence |
| Hybrid: relay deltas, persist milestones | Balance of replay and performance | Slightly more complex; define what a "milestone" is |

**Recommendation: Non-persisted delta relay.** Publish `stream_delta` events through the SSE broker but do NOT store them in the events table. When streaming completes, persist the final assembled message as a normal `message` event. This keeps the event store clean and performant. If crash recovery mid-stream is needed later, add checkpoint persistence (hybrid approach) -- but for v2.0, clean restart is acceptable.

---

## Sources

### Agent Tool Use / Function Calling
- [OpenAI Function Calling Guide](https://platform.openai.com/docs/guides/function-calling) -- MEDIUM confidence (403 on deep fetch; details from search summaries and SDK examination)
- [OpenAI Go SDK (openai-go v3.30.0)](https://github.com/openai/openai-go) -- HIGH confidence (official repository, version verified)
- [OpenAI Responses API Migration Guide](https://developers.openai.com/api/docs/guides/migrate-to-responses) -- MEDIUM confidence (search summary)
- [Tool Calling Explained: 2026 Guide (Composio)](https://composio.dev/content/ai-agent-tool-calling-guide) -- LOW confidence (third party)
- [OpenAI Practical Guide to Building Agents](https://openai.com/business/guides-and-resources/a-practical-guide-to-building-ai-agents/) -- MEDIUM confidence (official but marketing-adjacent)

### Web Search APIs for Agents
- [Agentic Search Benchmark 2026](https://aimultiple.com/agentic-search) -- MEDIUM confidence (independent benchmark with methodology)
- [Beyond Tavily: AI Search APIs in 2026](https://websearchapi.ai/blog/tavily-alternatives) -- MEDIUM confidence (comparison article)
- [Best Web Search APIs for AI 2026 (Firecrawl)](https://www.firecrawl.dev/blog/best-web-search-apis) -- MEDIUM confidence (vendor article)
- [Exa vs Tavily vs Serper vs Brave Comparison](https://dev.to/supertrained/exa-vs-tavily-vs-serper-vs-brave-search-for-ai-agents-an-score-comparison-2l1g) -- MEDIUM confidence (community benchmark)

### A2A Protocol
- [A2A Protocol Specification](https://a2a-protocol.org/latest/specification/) -- HIGH confidence (official spec, fetched and verified)
- [Project A2A deep dive doc](docs/superpowers/a2a-protocol-deep-dive.md) -- HIGH confidence (project documentation verified against spec)
- [A2A GitHub Repository](https://github.com/a2aproject/A2A) -- HIGH confidence (official)

### Artifact Rendering / UI
- [A2UI Protocol Guide (Google)](https://developers.googleblog.com/introducing-a2ui-an-open-project-for-agent-driven-interfaces/) -- MEDIUM confidence (nascent but authoritative source)
- [Streamdown (streaming markdown)](https://streamdown.ai/) -- MEDIUM confidence (specialized library for streaming markdown)
- [shadcn/ui AI Components](https://www.shadcn.io/ai) -- MEDIUM confidence (compatible with existing stack)
- [Vercel AI SDK Markdown Rendering](https://ai-sdk.dev/cookbook/next/markdown-chatbot-with-memoization) -- MEDIUM confidence (official Vercel docs)

### Streaming
- [OpenAI Streaming Chat Completions Events](https://developers.openai.com/api/reference/resources/chat/subresources/completions/streaming-events) -- HIGH confidence (official API reference)
- [OpenAI Streaming Responses Guide](https://developers.openai.com/api/docs/guides/streaming-responses) -- HIGH confidence (official docs)

### Inbound Webhooks
- [GitHub Webhooks Guide](https://www.magicbell.com/blog/github-webhooks-guide) -- MEDIUM confidence (well-documented standard pattern)
- [Slack Webhook Triggers](https://api.slack.com/automation/triggers/webhook) -- HIGH confidence (official Slack docs)
- [Hookdeck GitHub Agent Automation](https://hookdeck.com/webhooks/platforms/github-trigger-dev-claude-automation) -- MEDIUM confidence (practical implementation reference)

### Multi-Agent Framework Patterns
- [Multi-Agent Framework Comparison 2026](https://gurusup.com/blog/best-multi-agent-frameworks-2026) -- LOW confidence (overview article)
- [AI Agent Frameworks: CrewAI vs AutoGen vs LangGraph](https://designrevision.com/blog/ai-agent-frameworks) -- LOW confidence (comparison article)
