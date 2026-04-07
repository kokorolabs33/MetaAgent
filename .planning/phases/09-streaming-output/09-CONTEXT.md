# Phase 9: Streaming Output - Context

**Gathered:** 2026-04-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Agent replies stream token-by-token so users watch agents think in real time. Simplified architecture: agent uses OpenAI streaming API, tokens relayed through existing SSE broker as ephemeral delta events (not persisted). Frontend renders partial text progressively with react-markdown.

</domain>

<decisions>
## Implementation Decisions

### Streaming Architecture
- **D-01:** Simplified single-layer streaming — agent calls OpenAI with stream mode, receives token deltas, publishes each delta through the existing SSE broker to the frontend.
- **D-02:** Delta events are ephemeral — published via Broker.Publish() only, NOT saved to the event store. Only the final assembled message is persisted.
- **D-03:** New SSE event type: `agent.streaming_delta` with payload `{task_id, agent_id, delta_text, done}`. When `done: true`, the complete message follows as a normal `message` event.
- **D-04:** The streaming happens inside `cmd/openaiagent` — the A2A response is sent only after all tokens are collected, but deltas are published in real-time via a callback/webhook to the platform server.

### Frontend Rendering
- **D-05:** Each delta appends to an accumulated text buffer in Zustand store. Pass the full accumulated text to react-markdown on each update.
- **D-06:** Show a blinking cursor (CSS animation) at the end of the streaming message. Remove cursor when `done: true`.
- **D-07:** Streaming messages use the same ChatMessage component — just with partial text that grows over time.
- **D-08:** Progressive markdown rendering: react-markdown re-renders on each delta. Tables and code blocks may look incomplete during streaming — this is acceptable (they complete when streaming finishes).

### Claude's Discretion
- Exact cursor animation CSS
- Delta batch size (every token vs every N tokens)
- How to handle streaming + tool call interleave (if agent streams, calls tool, then streams again)
- SSE reconnection during active stream (show "reconnecting..." or replay from accumulated text)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Backend Streaming
- `cmd/openaiagent/openai.go` — Current OpenAI client, needs streaming mode
- `internal/events/broker.go` — Broker.Publish() for ephemeral delta events
- `internal/handlers/stream.go` — SSE handler (existing, handles new event types)

### Frontend
- `web/components/chat/ChatMessage.tsx` — Message rendering with react-markdown
- `web/lib/store.ts` — Zustand stores, handleEvent switch
- `web/lib/sse.ts` — SSE connection helpers

### Research
- `.planning/research/ARCHITECTURE.md` — Two-layer SSE analysis (we chose simplified)
- `.planning/research/PITFALLS.md` — Pitfall 3: broker 64-buffer incompatible with token streaming

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- SSE broker already supports publishing ephemeral events (just don't call EventStore.Save)
- react-markdown already installed and working for message rendering
- Zustand store pattern for accumulating state from SSE events

### Integration Points
- `cmd/openaiagent` — Add streaming OpenAI call, publish deltas to platform
- Platform server — New endpoint or callback to receive streaming deltas from agent
- Zustand store — New `streamingMessages` map for in-progress streams
- ChatMessage — Detect streaming state, show cursor

</code_context>

<specifics>
## Specific Ideas

- The "watching agent think" experience is the primary wow moment for streaming
- Cursor should be subtle — a thin blinking bar, not a heavy block cursor

</specifics>

<deferred>
## Deferred Ideas

- Full A2A tasks/sendSubscribe streaming — too complex for v2.0
- Streaming + tool call interleave — defer, handle sequentially (stream → tool → stream)
- Stream history replay on reconnect — just show final message

None — discussion stayed within phase scope

</deferred>

---

*Phase: 09-streaming-output*
*Context gathered: 2026-04-07*
