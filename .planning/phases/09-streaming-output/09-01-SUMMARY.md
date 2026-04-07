---
phase: 09-streaming-output
plan: 01
subsystem: api
tags: [openai, streaming, sse, a2a, broker]

# Dependency graph
requires:
  - phase: 07-agent-tool-use
    provides: OpenAI ChatWithTools method and tool definitions in openaiagent
provides:
  - Streaming OpenAI client (ChatWithToolsStream) with token batching
  - Platform streaming delta endpoint (POST /api/internal/streaming-delta)
  - Ephemeral agent.streaming_delta SSE event type via Broker.Publish
  - Executor streaming metadata injection for streaming-capable agents
  - X-Accel-Buffering headers on all SSE endpoints
affects: [09-streaming-output, frontend-streaming-ui]

# Tech tracking
tech-stack:
  added: []
  patterns: [streaming-sse-line-parser, token-batching-50ms-20chars, tool-call-accumulator-by-index, ephemeral-broker-event]

key-files:
  created:
    - cmd/openaiagent/stream.go
    - internal/handlers/streaming_delta.go
  modified:
    - cmd/openaiagent/openai.go
    - cmd/openaiagent/main.go
    - cmd/server/main.go
    - internal/executor/executor.go
    - internal/handlers/stream.go
    - internal/handlers/agent_status_stream.go
    - internal/handlers/conversations.go

key-decisions:
  - "Token batching at ~50ms/20chars reduces SSE event volume 10-20x while feeling real-time"
  - "Streaming deltas are ephemeral (Broker.Publish only, not EventStore.Save) per D-02"
  - "Platform metadata passed via A2A DataPart with _streaming_meta key for backward compatibility"

patterns-established:
  - "Ephemeral delta pattern: agent POSTs deltas to /api/internal/streaming-delta, handler publishes via Broker only"
  - "Streaming capability gating: executor checks agentHasCapability before injecting _streaming_meta"
  - "Token batching: accumulate in batchBuf, flush on 50ms ticker or 20-char threshold"

requirements-completed: [STRM-01]

# Metrics
duration: 5min
completed: 2026-04-07
---

# Phase 09 Plan 01: Streaming Output - Backend Pipeline Summary

**Streaming OpenAI client with token batching and platform relay endpoint for real-time agent.streaming_delta SSE events**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-07T06:03:37Z
- **Completed:** 2026-04-07T06:08:28Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- Streaming OpenAI client (ChatWithToolsStream) that handles both text-only and tool-call-then-text scenarios with proper delta accumulation
- Token batching (~50ms / 20 chars) prevents broker buffer overflow while maintaining real-time feel
- Platform streaming delta endpoint receives agent callbacks and relays as ephemeral SSE events
- Executor injects streaming metadata to streaming-capable agents via A2A DataPart
- X-Accel-Buffering: no added to all 4 SSE endpoints for proxy compatibility

## Task Commits

Each task was committed atomically:

1. **Task 1: Streaming OpenAI client and callback delivery in agent binary** - `3d4ce3d` (feat)
2. **Task 2: Platform streaming delta endpoint and executor integration** - `ca24452` (feat)

## Files Created/Modified
- `cmd/openaiagent/stream.go` - Streaming OpenAI client with ChatWithToolsStream, token batching, tool call accumulation, sendDelta callback
- `cmd/openaiagent/openai.go` - Added StreamDeltaHook type
- `cmd/openaiagent/main.go` - Platform URL config, platformMeta extraction from DataPart, streaming path wiring, agent card Streaming: true
- `internal/handlers/streaming_delta.go` - POST /api/internal/streaming-delta endpoint, ephemeral broker publish
- `cmd/server/main.go` - Route registration for streaming delta endpoint outside auth middleware
- `internal/executor/executor.go` - agentHasCapability helper, _streaming_meta DataPart injection
- `internal/handlers/stream.go` - X-Accel-Buffering header on Stream and MultiStream
- `internal/handlers/agent_status_stream.go` - X-Accel-Buffering header on agent status SSE
- `internal/handlers/conversations.go` - X-Accel-Buffering header on conversation SSE

## Decisions Made
- Token batching at ~50ms/20chars -- reduces event volume 10-20x vs per-token while maintaining real-time UX (per Pitfall 3 mitigation)
- Tool call arguments accumulated via index-keyed map, parsed only after stream completes (per Pitfall 2 prevention)
- Platform metadata passed as A2A DataPart with `_streaming_meta` key -- backward compatible with non-streaming callers (nil hook = silent skip)
- Streaming delta endpoint placed outside auth middleware as internal agent-to-platform callback

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added X-Accel-Buffering to conversation SSE endpoint**
- **Found during:** Task 2 (SSE header audit)
- **Issue:** Plan mentioned stream.go and agent_status_stream.go SSE endpoints but conversations.go also has an SSE endpoint missing the header
- **Fix:** Added X-Accel-Buffering: no to conversations.go Stream method
- **Files modified:** internal/handlers/conversations.go
- **Verification:** grep confirms 4 total X-Accel-Buffering headers across all SSE endpoints
- **Committed in:** ca24452 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Essential for proxy buffering prevention across all SSE endpoints. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Backend streaming pipeline complete: agent -> platform -> SSE broker
- Ready for Plan 09-02: frontend streaming UI (Zustand store for streaming messages, cursor animation, react-markdown progressive rendering)
- The openaiagent agent card now reports streaming: true, so when re-registered the executor will automatically inject streaming metadata

## Self-Check: PASSED

- All created files exist on disk
- All commit hashes found in git log
- SUMMARY.md created successfully

---
*Phase: 09-streaming-output*
*Completed: 2026-04-07*
