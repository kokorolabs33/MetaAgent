# Phase 4: Parallel Dashboard - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-05
**Phase:** 04-parallel-dashboard
**Areas discussed:** Dashboard Layout, SSE Connection Strategy, Task Filtering and Search

---

## Dashboard Layout

| Option | Description | Selected |
|--------|-------------|----------|
| Compact status card | Title + status badge + progress bar (3/5) + agent dots. No mini-DAG, clean and fast | ✓ |
| Mini-DAG card | Title + status + simplified DAG node blocks. More visual but larger cards | |
| Dense info card | Title + status + subtask list (agent + status per row). Most info but biggest cards | |

**User's choice:** Compact status card (recommended)
**Notes:** User preferred the clean, information-dense-but-compact approach over visual DAG representations in cards.

---

## SSE Connection Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Multiplexed SSE endpoint | New /api/tasks/stream?ids=a,b,c — one EventSource for all tasks. Backend fans out | ✓ |
| Independent SSE + polling fallback | Keep per-task SSE, fall back to REST polling when connection limit reached | |
| Rely on HTTP/2 | No changes, assume HTTP/2 multiplexing. May not work in local dev | |

**User's choice:** Multiplexed SSE endpoint (recommended)
**Notes:** Clean solution that works regardless of HTTP version. Only needs 1 SSE connection for all dashboard tasks.

---

## Task Filtering and Search

| Option | Description | Selected |
|--------|-------------|----------|
| Tab bar + search box | Status tabs (All/Running/Completed/Failed) + right-aligned search. Clear visual feedback | ✓ |
| Dropdown + search box | Status dropdown selector + search. More compact but less visual | |
| Filter chips | Clickable chips with multi-select. Supports combo filtering but more complex | |

**User's choice:** Tab bar + search box (recommended)
**Notes:** Simple, familiar pattern. One status active at a time via tabs.

---

## Claude's Discretion

- Grid responsive breakpoints and column count
- Progress bar styling
- Empty states per filter tab
- Pagination strategy
- Auto-refresh vs SSE-only updates
- Multiplexed SSE endpoint exact path format

## Deferred Ideas

- Mini-DAG in dashboard cards — decided against for v1
- Phase 3 (Templates) — skipped entirely by user request
