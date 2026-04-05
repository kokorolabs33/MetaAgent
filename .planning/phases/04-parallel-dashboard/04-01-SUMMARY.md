---
phase: 04-parallel-dashboard
plan: 01
subsystem: api
tags: [sse, broker, pgx, postgres, typescript, dashboard, multiplexing]

# Dependency graph
requires:
  - phase: 02-agent-status
    provides: events.Broker with SubscribeGlobal/UnsubscribeGlobal + agent.status_changed stream, used as reference pattern for the multiplexed fan-in
provides:
  - Task.CompletedSubtasks and Task.TotalSubtasks fields on the wire (snake_case JSON)
  - GET /api/tasks/stream?ids=a,b,c multiplexed SSE endpoint (authed)
  - scanTaskWithCounts helper in internal/handlers/tasks.go for List
  - TypeScript Task.completed_subtasks and Task.total_subtasks fields
affects: [04-02 frontend dashboard grid, 04-03 dashboard wiring, future phases that need bulk task progress]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Multiplexed SSE fan-in: N broker.Subscribe channels → per-sub goroutine → single merged channel → one HTTP writer"
    - "LEFT JOIN + GROUP BY for per-row aggregate counts in a list endpoint (no N+1)"
    - "Subscribe-before-replay for SSE, with no replay when the list API already carries the snapshot"

key-files:
  created:
    - internal/handlers/stream_test.go
  modified:
    - internal/models/task.go
    - internal/handlers/tasks.go
    - internal/handlers/stream.go
    - cmd/server/main.go
    - web/lib/types.ts
    - web/lib/conversationStore.ts

key-decisions:
  - "Cap multiplexed ids at 50 via maxMultiStreamIDs to bound per-request memory (~50 × 64 × sizeof(*Event))"
  - "Fan-in via N goroutines + one merged channel + single writer loop to avoid concurrent writes to http.ResponseWriter"
  - "No replay on /api/tasks/stream — the /api/tasks list response provides the snapshot; the stream only delivers live deltas"
  - "New scanTaskWithCounts helper instead of modifying scanTask so the Get path + a2a/server.go callers stay unchanged"
  - "LEFT JOIN + GROUP BY aggregates inside the List query to avoid N+1 per-card subtask lookups"

patterns-established:
  - "Multiplexed SSE endpoint pattern: validate+dedupe+cap ids → subscribe-before-work → deferred Unsubscribe loop → fan-in via wg + merged channel → single writer loop"
  - "Aggregate-augmented list query pattern: qualify all WHERE filters with the primary table alias, use LEFT JOIN for optional aggregates, GROUP BY primary key, COALESCE(SUM(CASE ...)) for conditional counts"

requirements-completed: [INTR-02, INTR-03]

# Metrics
duration: 6min
completed: 2026-04-05
---

# Phase 4 Plan 1: Backend Foundation for Parallel Dashboard Summary

**Per-task subtask counts in /api/tasks plus a multiplexed /api/tasks/stream?ids=a,b,c SSE endpoint that fans N broker subscriptions into one HTTP response writer.**

## Performance

- **Duration:** ~6 min
- **Started:** 2026-04-05T22:53:06Z
- **Completed:** 2026-04-05T22:59:14Z
- **Tasks:** 3 (2 code tasks + 1 verification task)
- **Files modified:** 6 (5 backend/shared + 1 frontend defaults fix)
- **Files created:** 1 (stream_test.go)

## Accomplishments

- `models.Task` gains `CompletedSubtasks int` and `TotalSubtasks int` with snake_case JSON tags (no `omitempty` — 0 is a valid rendered value)
- `TaskHandler.List` rewritten with `LEFT JOIN subtasks s ON s.task_id = t.id` + `GROUP BY t.id` returning both counts in a single query — no N+1
- New `scanTaskWithCounts` helper added; `scanTask` left untouched so `TaskHandler.Get` and `a2a/server.go` callers remain unchanged
- `StreamHandler.MultiStream` handles `GET /api/tasks/stream?ids=a,b,c` using subscribe-before-work + fan-in: validates, trims, dedupes, caps at 50 ids, subscribes per id, feeds a merged channel via per-sub goroutines, and writes from a single loop (avoids concurrent writes to `http.ResponseWriter`)
- Route registered inside the authenticated `r.Group(authMw.RequireAuth)` block in `cmd/server/main.go` so unauthenticated callers receive 401
- 5 unit tests covering empty ids, whitespace-only ids, overflow (51 ids), valid stream with published event, and duplicate-id deduplication — all passing
- TypeScript `Task` interface mirrors the Go JSON tags (`completed_subtasks: number; total_subtasks: number;`); `conversationStore.handleEvent` hydrates defaults for optimistic task creation

## Task Commits

1. **Task 1: Extend Task model + List query with subtask counts** - `268730d` (feat)
2. **Task 2: Add MultiStream SSE handler + register route + unit test** - `773823f` (feat)
3. **Task 3: Full backend quality gate (verification-only)** - no commit (no file modifications)

## Files Created/Modified

### Created
- `internal/handlers/stream_test.go` — 5 MultiStream unit tests (empty, whitespace-only, too many, valid streaming, dedupe)

### Modified
- `internal/models/task.go` — added `CompletedSubtasks` and `TotalSubtasks` fields at the end of the `Task` struct
- `internal/handlers/tasks.go` — rewrote `TaskHandler.List` query with LEFT JOIN + GROUP BY; qualified all WHERE filters with `t.`; added `scanTaskWithCounts` helper
- `internal/handlers/stream.go` — added `maxMultiStreamIDs` constant, `MultiStream` method, and `strings`/`sync` imports
- `cmd/server/main.go` — registered `r.Get("/api/tasks/stream", streamH.MultiStream)` inside the authed group, adjacent to the existing per-task stream
- `web/lib/types.ts` — added `completed_subtasks: number;` and `total_subtasks: number;` to the `Task` interface
- `web/lib/conversationStore.ts` — auto-fix: hydrated the two new required fields with `0` defaults in the optimistic Task object built from `task.created` events

## SQL Query Shape (for plan 02 reviewers)

```sql
SELECT
    t.id, t.title, t.description, t.status, t.created_by,
    t.metadata, t.plan, t.result, t.error, t.replan_count,
    t.created_at, t.completed_at,
    COALESCE(COUNT(s.id), 0) AS total_subtasks,
    COALESCE(SUM(CASE WHEN s.status = 'completed' THEN 1 ELSE 0 END), 0) AS completed_subtasks
FROM tasks t
LEFT JOIN subtasks s ON s.task_id = t.id
WHERE [t.status = $N] AND [(t.title ILIKE $N OR t.description ILIKE $N)]
GROUP BY t.id
ORDER BY t.created_at DESC
LIMIT $N OFFSET $N
```

Column order in the SELECT: `total_subtasks` comes before `completed_subtasks` because the SQL emits `COUNT(s.id)` before the `SUM(CASE ...)`. The `scanTaskWithCounts` destination order matches exactly (`&t.TotalSubtasks, &t.CompletedSubtasks`).

Count query (unchanged shape; aliased for consistency with WHERE):

```sql
SELECT COUNT(*) FROM tasks t [WHERE ...]
```

## MultiStream Handler Contract

**Signature:** `func (h *StreamHandler) MultiStream(w http.ResponseWriter, r *http.Request)`
**Route:** `GET /api/tasks/stream?ids=a,b,c` (inside `r.Group(authMw.RequireAuth)`)
**Headers on success:** `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`
**Events on the wire:** reuses `writeSSEEvent` verbatim — each frame is `id: {event_id}\ndata: {marshaled *models.Event}\n\n`. Since `models.Event.TaskID` already carries `json:"task_id"`, frontend consumers can dispatch per task without any new field.
**Error responses:**
- `400 {"error":"ids query param required"}` — empty or whitespace-only ids
- `400 {"error":"too many ids (max 50)"}` — >50 unique ids after dedupe
- `500 {"error":"streaming not supported"}` — ResponseWriter does not implement `http.Flusher`
- `401` — from auth middleware before the handler runs

## Authentication Confirmation

- `GET /api/tasks` registered at `cmd/server/main.go:197` inside `r.Group(func(r chi.Router) { r.Use(authMw.RequireAuth); ... })`
- `GET /api/tasks/stream` registered at `cmd/server/main.go:210` inside the same authed group (verified via `awk '/r.Group.*chi.Router/,/^\t\}\)/' cmd/server/main.go | grep -c "streamH.MultiStream"` = 1)

Unauthenticated callers receive 401 before either handler executes.

## Tests Added

All in `internal/handlers/stream_test.go`:

| Test | Coverage |
|------|----------|
| `TestMultiStream_EmptyIDs_Returns400` | Missing ids query param → 400 with the correct error message |
| `TestMultiStream_WhitespaceOnlyIDs_Returns400` | `ids=%20,%20` (whitespace-only after split) → 400 |
| `TestMultiStream_TooManyIDs_Returns400` | 51 comma-separated ids → 400 "too many ids" |
| `TestMultiStream_ValidIDs_StreamsPublishedEvents` | Valid two-id request → Content-Type/Cache-Control set, broker.Publish on task-1 lands a `"task_id":"task-1"` frame in the recorder, subtask.completed type round-trips |
| `TestMultiStream_DuplicateIDs_AreDeduplicated` | `ids=same,same,same` with one broker.Publish → exactly one `"id":"evt-x"` frame in the body (confirms dedupe prevents duplicate Subscribe) |

All 5 tests pass under `go test ./internal/handlers/... -run MultiStream -count=1`.

## Decisions Made

- **Cap at 50 ids per request** — comfortably exceeds the 12-per-page dashboard grid while bounding memory for T-04-02 DoS mitigation.
- **Fan-in via merged channel + single writer loop** — `net/http` requires serial writes to a `ResponseWriter`, so per-subscription goroutines feed a merged channel and only the main loop writes SSE frames.
- **No replay in the multiplexed stream** — the `/api/tasks` list response already provides the snapshot counts; adding replay would double-fetch and risk duplicate events on reconnect.
- **New `scanTaskWithCounts` instead of modifying `scanTask`** — the existing `scanTask` is shared with `TaskHandler.Get` and A2A server call sites; adding aggregate columns there would have required either an optional-scan shim or breaking those call sites.
- **Aggregated List query uses `t.`-qualified WHERE filters** — ambiguous column references would cause Postgres to error once the LEFT JOIN introduces `subtasks.status` into scope. Count query also switched to `FROM tasks t` so the same filter builder can serve both.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Hydrated new Task fields in conversationStore optimistic event handler**
- **Found during:** Task 1 (post-edit `tsc --noEmit` run)
- **Issue:** `web/lib/conversationStore.ts:154` builds an optimistic `Task` object on `task.created` events. After adding the two new required fields to the `Task` interface, TypeScript reported `TS2739: Type ... is missing the following properties from type 'Task': completed_subtasks, total_subtasks`. This was a direct compile error caused by the Task 1 interface change — not a pre-existing issue.
- **Fix:** Added `completed_subtasks: 0, total_subtasks: 0` to the optimistic object (a newly created task has no subtasks yet; the next snapshot refresh or SSE event will update them).
- **Files modified:** `web/lib/conversationStore.ts`
- **Verification:** `tsc --noEmit` exits 0 cleanly; all other worktree files typecheck.
- **Committed in:** `268730d` (Task 1 commit)

**2. [Rule 3 - Blocking] Symlinked node_modules from main repo into worktree for typecheck**
- **Found during:** Task 1 verify step
- **Issue:** The worktree at `.claude/worktrees/agent-a8186941/web/` had no `node_modules` directory, so `tsc --noEmit` reported dozens of unrelated `Cannot find module 'next'`, `'clsx'`, `'tailwind-merge'`, and `any`-inference errors in files I did not touch. These were environment errors, not code errors.
- **Fix:** Created a symlink from `.claude/worktrees/agent-a8186941/web/node_modules` → `/Users/jasper/Documents/code/kokorolabs33/taskhub/web/node_modules` so the existing tsc installation in the main repo resolves dependencies for the worktree. This is a local-dev convenience and does not affect committed code (no tracked files changed).
- **Files modified:** None tracked (symlink is outside git tracking under web/).
- **Verification:** After symlinking, `tsc --noEmit` narrowed to exactly the one legitimate error fixed in deviation #1 above, then exited 0.
- **Committed in:** N/A (environment fix, not a code change).

---

**Total deviations:** 2 auto-fixed (both Rule 3 - Blocking)
**Impact on plan:** Both auto-fixes were strictly necessary to make the Task 1 edit compile. Neither expands scope; the conversationStore change is a one-line defaults addition that keeps the optimistic event handler in sync with the new interface contract.

## Issues Encountered

- **Pre-existing lint/lint-env failures block `make check` end-to-end, but NOT on any file I modified.** Running `make check` revealed:
  - `make fmt-check-frontend` fails because `pnpm prettier --check .` reports `Command "prettier" not found` — prettier is not listed in `web/package.json` devDependencies. Pre-existing environment issue; out of scope per the scope boundary rule.
  - `make lint-backend` reports 12 issues in 4 files I did not touch: `cmd/openaiagent/main.go`, `internal/rbac/middleware.go`, `internal/executor/executor.go`, `internal/handlers/agents.go`. Pre-existing; out of scope.
  - **Zero lint issues on any file this plan modified.**
- Alternative verification for this plan was comprehensive regardless:
  - `gofmt -l` on all modified Go files: clean
  - `go vet ./internal/handlers/... ./internal/models/... ./cmd/server/...`: clean
  - `go build ./...`: exit 0
  - `go test ./...`: all packages pass (handlers, executor, a2a, models, events, audit, auth, orchestrator, policy, rbac, webhook, crypto, ctxutil, config)
  - `go test ./internal/handlers/... -run MultiStream`: 5/5 pass
  - `tsc --noEmit` in web/: exit 0

These pre-existing issues are logged here for visibility but will not be fixed by this plan.

## Deferred Items

- Fix prettier availability in `web/package.json` (add to devDependencies, or install via `pnpm add -D prettier`) so `make fmt-check-frontend` can run. Logged here; not touched.
- Fix the 12 pre-existing `golangci-lint` findings in `cmd/openaiagent`, `internal/rbac`, `internal/executor`, `internal/handlers/agents.go`. Logged here; not touched.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- **Unblocks Plan 02 and Plan 03** of this phase: frontend dashboard plans can now import the shared Task interface with compile-time guarantees that `completed_subtasks` and `total_subtasks` exist on every task list item.
- **Frontend SSE client** (plans 02/03) can point an `EventSource` at `GET /api/tasks/stream?ids=a,b,c` and dispatch events by the existing `event.task_id` field — no new field needed, no format breaking changes.
- **Progress bar UI** can be fed from `task.completed_subtasks / task.total_subtasks` directly — no extra API calls, no N+1 lookups.
- No open blockers for plans 02 or 03.

---
*Phase: 04-parallel-dashboard*
*Completed: 2026-04-05*

## Self-Check: PASSED

- All 7 modified/created source files exist on disk
- Both commits (`268730d`, `773823f`) exist in `git log --oneline --all`
- SUMMARY.md exists at `.planning/phases/04-parallel-dashboard/04-01-SUMMARY.md`
