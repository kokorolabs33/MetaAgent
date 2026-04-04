---
phase: 01-foundation
plan: 01
subsystem: backend-core
tags: [sse, race-condition, anthropic-sdk, llm-integration]
dependency_graph:
  requires: []
  provides: [race-free-sse, anthropic-sdk-integration]
  affects: [internal/handlers/stream.go, internal/orchestrator/orchestrator.go, cmd/server/main.go, go.mod]
tech_stack:
  added: [anthropic-sdk-go@v1.30.0]
  patterns: [subscribe-before-replay, event-dedup-by-id]
key_files:
  created: []
  modified:
    - internal/handlers/stream.go
    - internal/orchestrator/orchestrator.go
    - cmd/server/main.go
    - go.mod
    - go.sum
decisions:
  - Used Claude Sonnet 4.5 model constant (ModelClaudeSonnet4_5) as default model for orchestrator LLM calls
  - Dedup map entries deleted after first hit rather than time-based cleanup for simpler bounded memory
metrics:
  duration: 6m
  completed: "2026-04-04T22:46:00Z"
  tasks_completed: 2
  tasks_total: 2
---

# Phase 01 Plan 01: Fix SSE Race + Replace Claude CLI Summary

Race-free SSE streaming via subscribe-before-replay with event dedup, plus Anthropic Go SDK replacing claude CLI subprocess for Docker-compatible LLM calls.

## Task Results

### Task 1: Fix SSE subscribe/replay race condition (FOUND-02)

**Commit:** `3423e61`
**Files:** `internal/handlers/stream.go`

Rewrote the `Stream` handler to subscribe to the broker channel BEFORE querying the database for historical events. Events published during the replay window are deduplicated by tracking replayed event IDs in a `map[string]struct{}`. Dedup entries are deleted after their first hit to bound memory. This eliminates the race window where events could be published between the replay query and the subscribe call, which caused frozen DAG nodes on the frontend.

Key changes:
- `Broker.Subscribe(taskID)` moved from line 73 to line 43 (before any DB query)
- Added `seen` dedup map initialized from replay event IDs
- Live events check the dedup set before writing to SSE stream
- `writeSSEEvent` and broker/store interfaces untouched

### Task 2: Replace claude CLI with Anthropic Go SDK (FOUND-01)

**Commit:** `55437e9`
**Files:** `internal/orchestrator/orchestrator.go`, `cmd/server/main.go`, `go.mod`, `go.sum`

Replaced `exec.CommandContext("claude", ...)` subprocess invocation with the Anthropic Go SDK (`anthropic-sdk-go v1.30.0`). Added `APIKey` field to the `Orchestrator` struct, populated from `config.AnthropicAPIKey` at construction in `cmd/server/main.go`. All three callers (`Plan`, `Replan`, `DetectIntent`) pass `o.APIKey` to `callLLM`. When the API key is empty, `callLLM` returns a clear error: "ANTHROPIC_API_KEY is not set -- task execution requires an Anthropic API key".

Key changes:
- Removed `os/exec` import, added `anthropic-sdk-go` and `option` imports
- `callLLM` now takes `apiKey` as second parameter, creates SDK client, calls `Messages.New`
- Response content extracted by iterating `message.Content` blocks where `Type == "text"`
- `stripMarkdownFences` retained as safety net
- All existing tests pass unchanged

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] TextBlockParam type mismatch in SDK**
- **Found during:** Task 2
- **Issue:** Plan suggested `anthropic.NewTextBlock(systemPrompt)` for the System field, but `System` expects `[]anthropic.TextBlockParam` not `[]ContentBlockParamUnion`
- **Fix:** Used struct literal `{Text: systemPrompt}` instead of `NewTextBlock()` helper
- **Files modified:** `internal/orchestrator/orchestrator.go`
- **Commit:** `55437e9`

## Verification Results

| Check | Result |
|-------|--------|
| `go build ./...` | PASS |
| Subscribe before ListByTask in stream.go | PASS (line 43 vs line 57) |
| No `exec.CommandContext` in orchestrator.go | PASS (0 matches) |
| `anthropics/anthropic-sdk-go` in go.mod | PASS (v1.30.0) |
| Clear error for missing API key | PASS |
| Orchestrator tests pass | PASS (7/7) |

## Threat Mitigations Verified

| Threat ID | Mitigation | Verified |
|-----------|------------|----------|
| T-01-02 | API key not logged anywhere in orchestrator.go | Yes |
| T-01-03 | Dedup map bounded by replay count, entries deleted after hit | Yes |

## Self-Check: PASSED

All 6 files verified on disk. Both commit hashes (3423e61, 55437e9) found in git log.
