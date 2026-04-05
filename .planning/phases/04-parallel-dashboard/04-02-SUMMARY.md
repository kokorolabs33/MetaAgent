---
phase: 04-parallel-dashboard
plan: 02
subsystem: ui
tags: [react, typescript, tailwind, shadcn, dashboard, components, accessibility]

# Dependency graph
requires:
  - phase: 04-parallel-dashboard/01
    provides: Task.completed_subtasks and Task.total_subtasks fields in web/lib/types.ts
  - phase: 02-agent-status
    provides: AgentStatusDot component for real-time agent status visualization
provides:
  - SubtaskProgressBar component with conditional color (amber/blue/green/red) and ARIA progressbar role
  - DashboardTaskCard composing Card + Badge + SubtaskProgressBar + AgentStatusDot with click navigation
  - DashboardEmptyState with 5 copy variants matching UI-SPEC copywriting contract
  - TaskFilterBar with URL-synced tabs (All/Running/Completed/Failed) + debounced search + NewTaskDialog CTA
affects: [04-03 dashboard wiring and page orchestration, future dashboard enhancements]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Callback-injected status resolver: DashboardTaskCard receives getAgentStatus callback instead of calling useAgentStore directly, keeping the card pure and unit-testable"
    - "URL-as-source-of-truth for filters: TaskFilterBar reads/writes URL params via useSearchParams + router.replace, no local filter state"
    - "Debounced search input: local useState + useEffect setTimeout(300ms) pattern for URL write"
    - "Verbatim statusConfig copy: DashboardTaskCard duplicates statusConfig from TaskCard.tsx to keep both cards in visual lockstep without coupling exports"

key-files:
  created:
    - web/components/dashboard/SubtaskProgressBar.tsx
    - web/components/dashboard/DashboardTaskCard.tsx
    - web/components/dashboard/DashboardEmptyState.tsx
    - web/components/dashboard/TaskFilterBar.tsx
  modified: []

key-decisions:
  - "Callback injection for agent status resolver instead of direct store access — keeps DashboardTaskCard pure/testable and avoids re-renders from unrelated agent store updates"
  - "statusConfig and timeAgo copied verbatim from TaskCard.tsx rather than extracting to shared module — matches UI-SPEC mandate and avoids touching existing TaskCard exports"
  - "MAX_VISIBLE_DOTS=5 with +N overflow label for agent dots per UI-SPEC agent overflow contract"
  - "Search debounce at 300ms matching UI-SPEC Interaction Contract specification"

patterns-established:
  - "Callback-injected status resolver pattern for pure presentational cards that need store data"
  - "URL-synced filter bar pattern: useSearchParams read + router.replace write + page reset on filter change"

requirements-completed: [INTR-02, INTR-04]

# Metrics
duration: 3min
completed: 2026-04-05
---

# Phase 4 Plan 2: Dashboard UI Components Summary

**Four presentational dashboard components: SubtaskProgressBar with conditional ARIA-accessible color, DashboardTaskCard composing Card + Badge + AgentStatusDot + progress, TaskFilterBar with URL-synced tabs and debounced search, DashboardEmptyState with 5 copy variants.**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-04-05T23:04:25Z
- **Completed:** 2026-04-05T23:07:28Z
- **Tasks:** 3
- **Files created:** 4

## Accomplishments

- SubtaskProgressBar renders conditional color bar (amber < 50%, blue 50-99%, green 100%, red if failed) with `role="progressbar"`, `aria-valuenow`, `aria-valuemax`, and `aria-label="Subtask progress"`; label reads "{completed}/{total} subtasks"
- DashboardTaskCard composes Card + Badge + SubtaskProgressBar + AgentStatusDot row with exact statusConfig copy from TaskCard.tsx; navigates on click and Enter/Space; max 5 agent dots with +N overflow
- DashboardEmptyState provides all 5 copy variants from UI-SPEC copywriting contract verbatim (all, running, completed, failed, search)
- TaskFilterBar reads URL via useSearchParams, writes via router.replace, debounces search at 300ms, renders ARIA tablist/tab roles with aria-selected, and mounts NewTaskDialog CTA

## Task Commits

1. **Task 1: SubtaskProgressBar + DashboardEmptyState** - `f213dbb` (feat)
2. **Task 2: DashboardTaskCard** - `9a226a0` (feat)
3. **Task 3: TaskFilterBar** - `d7cf5e1` (feat)

## Files Created

- `web/components/dashboard/SubtaskProgressBar.tsx` — Progress bar with conditional color + ARIA progressbar role + "{completed}/{total} subtasks" label
- `web/components/dashboard/DashboardEmptyState.tsx` — Per-filter empty state with 5 copy variants (all/running/completed/failed/search)
- `web/components/dashboard/DashboardTaskCard.tsx` — Enhanced card composing Card + Badge + SubtaskProgressBar + AgentStatusDot with click/keyboard navigation
- `web/components/dashboard/TaskFilterBar.tsx` — Tab bar (All/Running/Completed/Failed) + debounced search input + NewTaskDialog CTA, URL-synced via useSearchParams

## Component Export and Props Summary

### SubtaskProgressBar

```typescript
export function SubtaskProgressBar({ completed, total, failed }: {
  completed: number;
  total: number;
  failed?: boolean;
})
```

### DashboardTaskCard

```typescript
export function DashboardTaskCard({ task, progress, agentIds, getAgentStatus }: {
  task: Task;
  progress?: { completed: number; total: number };
  agentIds: string[];
  getAgentStatus: (agentId: string) => AgentActivityStatus;
})
```

### DashboardEmptyState

```typescript
export type DashboardEmptyStateVariant = "all" | "running" | "completed" | "failed" | "search";
export function DashboardEmptyState({ variant }: { variant: DashboardEmptyStateVariant })
```

### TaskFilterBar

```typescript
type TabKey = "all" | "running" | "completed" | "failed";
export function TaskFilterBar({ counts }: {
  counts?: Partial<Record<TabKey, number>>;
})
```

## Import Graph (for Plan 03)

```
TaskFilterBar
  <- @/components/ui/input (Input)
  <- @/components/dashboard/NewTaskDialog (NewTaskDialog)
  <- @/lib/utils (cn)
  <- next/navigation (useSearchParams, usePathname, useRouter)
  <- lucide-react (Search)

DashboardTaskCard
  <- @/components/ui/card (Card, CardHeader, CardTitle, CardContent)
  <- @/components/ui/badge (Badge)
  <- @/components/agent/AgentStatusDot (AgentStatusDot)
  <- @/components/dashboard/SubtaskProgressBar (SubtaskProgressBar)
  <- @/lib/types (Task, AgentActivityStatus)
  <- @/lib/utils (cn — used indirectly via SubtaskProgressBar)
  <- next/navigation (useRouter)

SubtaskProgressBar
  <- @/lib/utils (cn)

DashboardEmptyState
  <- lucide-react (Inbox)
```

## statusConfig Verification

The `statusConfig` object in `DashboardTaskCard.tsx` is a verbatim copy of the one in `TaskCard.tsx:8-40`. All 7 status entries match exactly: pending, planning, running, completed, failed, cancelled, approval_required. The `timeAgo` function is also copied verbatim from `TaskCard.tsx:42-55`.

## Decisions Made

- **Callback injection for agent status** — DashboardTaskCard receives a `getAgentStatus` callback prop instead of importing useAgentStore directly. This keeps the card pure/testable and avoids re-renders from unrelated agent store updates. Plan 03's TaskDashboard will pass `useAgentStore.getState().getAgentStatus` as the callback.
- **Verbatim copy of statusConfig and timeAgo** — Rather than extracting these to a shared module (which would require modifying TaskCard.tsx exports), both are copied in full with a comment linking to the source. This matches the UI-SPEC mandate that the two card variants stay in visual lockstep.
- **Optional progress prop with task field fallback** — DashboardTaskCard accepts an optional `progress` prop from the SSE-updated dashboard store but falls back to `task.completed_subtasks` / `task.total_subtasks` from the list API. This means cards render meaningful progress even before SSE delivers any deltas.
- **Optional counts prop on TaskFilterBar** — Tab counts are optional so the component works standalone. Plan 03 will compute and pass counts from the analytics or list response.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **Unblocks Plan 03:** All 4 components are importable and renderable. Plan 03 only needs to mount TaskDashboard (page-level orchestrator) composing TaskFilterBar + DashboardTaskCard grid + DashboardEmptyState, wire SSE via connectMultiTaskSSE, and replace the EmptyState at `/`.
- **No open blockers.** All components pass TypeScript strict mode and ESLint no-explicit-any.
- **No stubs.** All components render real data from props; no hardcoded empty values or placeholder text.

## Self-Check: PASSED

- All 4 created component files exist on disk
- All 3 commits (`f213dbb`, `9a226a0`, `d7cf5e1`) exist in `git log --oneline --all`
- SUMMARY.md exists at `.planning/phases/04-parallel-dashboard/04-02-SUMMARY.md`
- Zero `: any` or `as any` occurrences in any new file
- TypeScript strict mode typecheck passes
- ESLint passes on all 4 new files

---
*Phase: 04-parallel-dashboard*
*Completed: 2026-04-05*
