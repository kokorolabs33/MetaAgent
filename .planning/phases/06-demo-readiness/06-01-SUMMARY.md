---
phase: 06-demo-readiness
plan: 01
subsystem: api
tags: [openai, llm, orchestrator, gpt-4o-mini]

# Dependency graph
requires: []
provides:
  - "internal/llm package: shared OpenAI Chat Completions client"
  - "Orchestrator uses OpenAI API instead of claude CLI binary"
  - "OpenAIAPIKey config field loaded from environment"
affects: [06-demo-readiness]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Shared LLM client via dependency injection (internal/llm.Client)"
    - "Startup warning for missing API keys (non-fatal)"

key-files:
  created:
    - "internal/llm/openai.go"
  modified:
    - "internal/orchestrator/orchestrator.go"
    - "internal/config/config.go"
    - "cmd/server/main.go"
    - "cmd/openaiagent/openai.go"
    - "cmd/openaiagent/main.go"

key-decisions:
  - "D-01: Raw HTTP OpenAI client (no SDK library) -- matches existing cmd/openaiagent pattern"
  - "D-02: Default model gpt-4o-mini -- cost-effective for task decomposition"
  - "D-03: Clean break from claude CLI -- no dual-provider fallback"

patterns-established:
  - "internal/llm.Client as the shared LLM interface for all server-side LLM calls"
  - "API key injected via config, not read from env at call site"

requirements-completed: [DEMO-01]

# Metrics
duration: 4min
completed: 2026-04-07
---

# Phase 06 Plan 01: OpenAI LLM Client Extraction Summary

**Replaced claude CLI exec in orchestrator with shared OpenAI Chat Completions client (gpt-4o-mini) via internal/llm package**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-07T02:34:25Z
- **Completed:** 2026-04-07T02:38:39Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Extracted OpenAI HTTP client from cmd/openaiagent into reusable internal/llm package
- Replaced claude CLI binary dependency in orchestrator with OpenAI API calls
- Server logs a startup warning when OPENAI_API_KEY is not set (non-fatal, graceful degradation)

## Task Commits

Each task was committed atomically:

1. **Task 1: Extract OpenAI client into internal/llm and add config field** - `0bc8974` (feat)
2. **Task 2: Replace callLLM in orchestrator with llm.Client and wire in main.go** - `69c2e63` (feat)
3. **Housekeeping: Add openaiagent binary to gitignore** - `54cd0ac` (chore)

## Files Created/Modified
- `internal/llm/openai.go` - Shared OpenAI Chat Completions client with Client, ChatMessage, Chat, ChatWithHistory, Model
- `internal/orchestrator/orchestrator.go` - Replaced callLLM from exec.CommandContext("claude",...) to llm.Client.Chat delegation
- `internal/config/config.go` - Added OpenAIAPIKey field loaded from OPENAI_API_KEY env var
- `cmd/server/main.go` - Wires llm.NewClient into Orchestrator, logs warning if key missing
- `cmd/openaiagent/openai.go` - Refactored to delegate to internal/llm.Client (wrapper pattern)
- `cmd/openaiagent/main.go` - Updated model accessor from direct field to client.client.Model()
- `.gitignore` - Added /openaiagent binary

## Decisions Made
- Used raw HTTP client (no official OpenAI Go SDK) to match existing codebase pattern and avoid new dependencies
- Default model set to gpt-4o-mini (overridable via OPENAI_MODEL env var) for cost-effective task decomposition
- Clean break from claude CLI -- no fallback or dual-provider support; simplifies codebase

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added Model() accessor to llm.Client**
- **Found during:** Task 1 (cmd/openaiagent build)
- **Issue:** cmd/openaiagent/main.go accessed `client.model` directly; after wrapping, the unexported field was inaccessible
- **Fix:** Added `Model() string` method to llm.Client, updated main.go to use `client.client.Model()`
- **Files modified:** `internal/llm/openai.go`, `cmd/openaiagent/main.go`
- **Verification:** `go build ./cmd/openaiagent/` passes
- **Committed in:** 0bc8974 (Task 1 commit)

**2. [Rule 3 - Blocking] Added openaiagent to .gitignore**
- **Found during:** Post-Task 2 cleanup
- **Issue:** `go build ./cmd/openaiagent/` produced untracked binary not in .gitignore
- **Fix:** Added `/openaiagent` to .gitignore following existing pattern
- **Files modified:** `.gitignore`
- **Committed in:** 54cd0ac

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both fixes necessary for build correctness and clean git state. No scope creep.

## Issues Encountered
- gofmt formatting was needed on cmd/server/main.go after adding the llm import -- caught by pre-commit hook, fixed inline before commit

## Deferred Items
- `internal/handlers/evolution.go` still uses `exec.CommandContext(ctx, "claude", ...)` -- this is a pre-existing reference outside the orchestrator package, not in scope for this plan

## User Setup Required
None - no external service configuration required. Users need OPENAI_API_KEY set in their environment (documented via startup warning).

## Next Phase Readiness
- internal/llm package available for any future LLM calls
- Orchestrator fully operational with OpenAI API
- evolution.go still uses claude CLI (separate concern, not blocking demo readiness)

---
*Phase: 06-demo-readiness*
*Completed: 2026-04-07*
