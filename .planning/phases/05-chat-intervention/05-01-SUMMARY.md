---
phase: 05-chat-intervention
plan: 01
subsystem: api
tags: [a2a, advisory, sse, broker, typing-indicator, executor]

# Dependency graph
requires:
  - phase: 02-agent-status
    provides: Agent status tracking, Broker pub/sub infrastructure
provides:
  - SendAdvisory method on DAGExecutor for advisory A2A messaging
  - routeAdvisory handler method with D-02 active-agent validation
  - publishTransientEvent for Broker-only ephemeral SSE events
  - advisory_errors response field in POST /tasks/{id}/messages
affects: [05-02, frontend-chat, typing-indicators]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Transient Broker-only events (publishTransientEvent) for ephemeral SSE data that should not be replayed on reconnect"
    - "Advisory pipeline isolation: SendAdvisory creates own context.Background() with timeout, never inherits caller context"
    - "Advisory metadata on messages: metadata JSONB column carries {advisory: true, advisory_for_subtask: id}"
    - "Validation errors returned as additional response data, not HTTP error codes (message always preserved)"

key-files:
  created: []
  modified:
    - internal/executor/executor.go
    - internal/handlers/messages.go

key-decisions:
  - "SendAdvisory is a separate method from SendFollowUp -- SendFollowUp mutates subtask status which would crash the DAG executor if used for advisory"
  - "SendAdvisory takes NO ctx parameter -- creates context.WithTimeout(context.Background(), 60s) internally per D-16 to prevent cancellation propagation"
  - "Typing indicators use Broker.Publish only (not EventStore.Save) -- transient events not replayed on SSE reconnect"
  - "Advisory errors returned as advisory_errors field alongside the saved message, not as HTTP 400 -- user message is never discarded"

patterns-established:
  - "publishTransientEvent: Broker-only event publishing for ephemeral SSE data"
  - "Advisory metadata pattern: messages.metadata carries {advisory: true, advisory_for_subtask: subtaskID}"
  - "Validation-error-in-response: returning validation info alongside the primary resource instead of rejecting the request"

requirements-completed: [INTR-01]

# Metrics
duration: 4min
completed: 2026-04-07
---

# Phase 5 Plan 01: Advisory Messaging Backend Pipeline Summary

**SendAdvisory A2A pipeline with background-context isolation, Broker-only typing indicators, and D-02 active-agent validation returning advisory_errors**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-07T00:35:28Z
- **Completed:** 2026-04-07T00:39:11Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Added 4 new methods to DAGExecutor: SendAdvisory, publishAdvisoryMessage, publishAdvisoryError, publishTransientEvent
- Replaced routeToAgents with routeAdvisory that validates agent activity status and returns D-02 errors
- Advisory messages are completely isolated from DAG execution -- never touch subtasks table
- Typing indicator events (agent.typing, agent.typing_stopped) flow via Broker-only transient events

## Task Commits

Each task was committed atomically:

1. **Task 1: Add SendAdvisory pipeline to DAGExecutor** - `21c4ab6` (feat)
2. **Task 2: Refactor MessageHandler.routeToAgents for advisory routing with D-02 validation** - `adf13fc` (feat)

## Files Created/Modified
- `internal/executor/executor.go` - Added SendAdvisory (advisory A2A dispatch with 60s timeout, retry-once), publishAdvisoryMessage (message with advisory metadata), publishAdvisoryError (inline error via system message), publishTransientEvent (Broker-only ephemeral SSE)
- `internal/handlers/messages.go` - Replaced routeToAgents with routeAdvisory (D-02 validation, returns errors for inactive agents), updated Send handler to include advisory_errors in response, removed SendFollowUp dependency

## Decisions Made
- **SendAdvisory vs SendFollowUp:** Created a completely separate method rather than modifying SendFollowUp. SendFollowUp (line 832-862) mutates subtask status to completed/input_required/failed on response, which would crash the DAG executor for advisory messages. This follows the D-05 reinterpretation documented in RESEARCH.md.
- **No ctx parameter on SendAdvisory:** The method creates its own `context.WithTimeout(context.Background(), 60s)` to guarantee D-16 isolation. Callers cannot accidentally pass the HTTP request context.
- **Broker-only typing indicators:** publishTransientEvent constructs Event structs manually and calls Broker.Publish directly, bypassing EventStore.Save entirely. This prevents typing indicator events from being replayed on SSE reconnect.
- **Advisory errors as response enrichment:** When an @mentioned agent is inactive, the error is returned as `advisory_errors` alongside the saved message (HTTP 201), not as an HTTP 400. The user's message is always preserved in the DB.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Removed unused `log` import from messages.go**
- **Found during:** Task 2 (routeAdvisory refactor)
- **Issue:** After removing the `routeToAgents` method which contained the only `log.Printf` call, the `log` import became unused causing a compilation error
- **Fix:** Removed the unused `log` import
- **Files modified:** `internal/handlers/messages.go`
- **Verification:** `go build ./internal/handlers/` succeeds
- **Committed in:** adf13fc (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Minor cleanup necessary for compilation. No scope creep.

## Issues Encountered
None

## Threat Flags

No new security surfaces introduced beyond those documented in the plan's threat model. All SQL queries use parameterized placeholders. Advisory content is wrapped with clear prefixes. Typing indicators are ephemeral and carry no sensitive data.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Backend advisory pipeline complete and ready for frontend consumption (Plan 02)
- POST /tasks/{id}/messages now returns `advisory_errors` for the frontend to handle
- SSE events `agent.typing` and `agent.typing_stopped` are published for typing indicator UI
- Advisory response messages carry `metadata.advisory: true` for visual distinction in ChatMessage

## Self-Check: PASSED

All files exist, all commits verified, all methods present, server builds cleanly.

---
*Phase: 05-chat-intervention*
*Completed: 2026-04-07*
