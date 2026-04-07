# Phase 7: Agent Tool Use - Context

**Gathered:** 2026-04-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Agents can call tools during task execution via OpenAI function calling. Web search (Tavily) is the first tool. Users see tool call activity in real time via SSE events in the chat feed. Different agent roles have different available tool sets.

</domain>

<decisions>
## Implementation Decisions

### Tool Definition Strategy
- **D-01:** Tools are hardcoded in Go with a role-to-toolset mapping. Each agent role (Engineering, Marketing, etc.) has a predefined set of available tools.
- **D-02:** No database or config file for tool definitions — code-level registration is sufficient for demo.

### Web Search Implementation
- **D-03:** Use Tavily API as the sole search provider. Hand-rolled Go HTTP client (single POST endpoint, no SDK needed).
- **D-04:** `TAVILY_API_KEY` env var required. If not set, web search tool is unavailable (not a fatal error — agent works without it).
- **D-05:** Tavily returns pre-summarized LLM-optimized responses — pass directly to agent context without post-processing.

### Tool Call Visualization
- **D-06:** Tool calls display as inline status lines in the chat feed: "Searching: [query]..." → "Search complete" with SSE events.
- **D-07:** New SSE event types: `tool.call_started` (tool name + args) and `tool.call_completed` (tool name + summary).
- **D-08:** Frontend renders tool status as a compact inline element in the message flow, not a separate card.

### ChatMessage Redesign
- **D-09:** Minimal change — only extend `cmd/openaiagent`'s `chatMessage` struct with `tool_calls` and `tool_call_id` fields. `internal/llm`'s ChatMessage stays unchanged (orchestrator doesn't need tool use).
- **D-10:** Tool call loop runs inside `cmd/openaiagent` — call LLM → if tool_calls in response → execute tools → append tool results → call LLM again → repeat until no more tool_calls.

### Per-Agent Tool Sets
- **D-11:** Define a `ToolSet` map keyed by agent role name. Each entry lists available tool definitions.
- **D-12:** Seed agents with role-appropriate tools: e.g., all agents get web_search; Engineering also gets code_analysis (future); Marketing gets competitor_search (future).
- **D-13:** For v2.0, web_search is the only implemented tool. Other tools are defined in the toolset but return "not yet implemented" — this shows the extensibility without requiring full implementation.

### Claude's Discretion
- Tool execution timeout and retry strategy
- Exact SSE event payload structure for tool events
- Tavily API request parameters (search_depth, include_answer, etc.)
- How to handle tool call failures gracefully in the chat flow

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### LLM Client
- `internal/llm/openai.go` — Current OpenAI client with Chat and ChatWithHistory (no tool support)
- `cmd/openaiagent/openai.go` — Team agent's OpenAI client (primary extension target for tool_calls)
- `cmd/openaiagent/roles.go` — Agent role definitions with system prompts

### A2A Protocol
- `internal/a2a/protocol.go` — A2A message format with TextPart and ArtifactPart
- `cmd/openaiagent/main.go` — A2A HTTP server handling tasks/send

### SSE Infrastructure
- `internal/events/broker.go` — Event broker with Subscribe/Publish
- `internal/events/store.go` — Event store for persistence
- `web/lib/sse.ts` — Frontend SSE helpers
- `web/lib/store.ts` — Zustand stores with SSE event handling

### Frontend Chat
- `web/components/chat/ChatMessage.tsx` — Message rendering
- `web/components/chat/GroupChat.tsx` — Chat feed container

### Research
- `.planning/research/STACK.md` — OpenAI Go SDK v3.30.0 recommendation
- `.planning/research/PITFALLS.md` — Pitfall 1: tool call conversation history struct design
- `.planning/research/ARCHITECTURE.md` — Tool use integration architecture

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/openaiagent/openai.go` — Existing OpenAI client to extend with tool support
- `internal/events/broker.go` — SSE broker for new tool.call events
- Agent role system in `cmd/openaiagent/roles.go` — natural place for tool set mapping

### Established Patterns
- SSE event publishing via EventStore.Save() + Broker.Publish()
- Zustand store handleEvent switch on event.type
- A2A message parts (TextPart, ArtifactPart) for structured responses

### Integration Points
- `cmd/openaiagent/openai.go` — Add ChatWithTools method and tool call loop
- `cmd/openaiagent/main.go` — Wire tool execution into A2A task handler
- SSE broker — New event types for tool call visibility
- Frontend chat — New inline component for tool call status

</code_context>

<specifics>
## Specific Ideas

- The killer demo: ask "Research competitor pricing for [product X]" → watch agent search in real time → get real data
- Tool call status should feel lightweight — a small status line, not a heavy card

</specifics>

<deferred>
## Deferred Ideas

- Code analysis tool — future, needs sandboxed execution
- File read/write tools — future, needs security model
- Parallel tool calls — defer, sequential is sufficient for demo
- Tool execution metadata in analytics — nice to have, not v2.0

None — discussion stayed within phase scope

</deferred>

---

*Phase: 07-agent-tool-use*
*Context gathered: 2026-04-07*
