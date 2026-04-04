---
phase: 1
slug: foundation
status: draft
shadcn_initialized: true
preset: base-nova
created: 2026-04-04
---

# Phase 1 — UI Design Contract

> Visual and interaction contract for Phase 1 (Foundation). Scoped to TraceTimeline integration, replanning event visibility, and bug fixes. Not a design system overhaul.

---

## Design System

| Property | Value |
|----------|-------|
| Tool | shadcn |
| Preset | base-nova |
| Component library | base-ui (@base-ui/react 1.3.0) |
| Icon library | lucide-react 0.577.0 |
| Font | Inter (Google Fonts, loaded via next/font) |
| CSS framework | Tailwind CSS v4 (via @tailwindcss/postcss) |
| Theme mode | Dark only (html class="dark" hardcoded in layout.tsx) |

**Source:** `web/components.json`, `web/app/layout.tsx`, `web/app/globals.css`

---

## Spacing Scale

Declared values (must be multiples of 4). These match the existing codebase patterns observed in task detail page and TraceTimeline:

| Token | Value | Usage |
|-------|-------|-------|
| xs | 4px | Icon gaps, timeline dot ring offset, inline padding (Tailwind: `gap-1`, `p-1`) |
| sm | 8px | Compact element spacing, badge gaps, between-event spacing (Tailwind: `gap-2`, `p-2`) |
| md | 16px | Default element spacing, panel padding, section gaps (Tailwind: `gap-4`, `p-4`) |
| lg | 24px | Section padding, header vertical padding (Tailwind: `gap-6`, `p-6`) |
| xl | 32px | Layout gaps, major section breaks (Tailwind: `gap-8`, `p-8`) |

Exceptions:
- Timeline dot size: 9px (`size-[9px]`) -- inherited from existing TraceTimeline, keep as-is
- Timeline vertical line: 1px width (`w-px`) -- standard border width
- Touch targets on expand/collapse buttons: minimum 24px hit area (existing pattern)

---

## Typography

All values match existing codebase patterns. Do not change font sizes for Phase 1.

| Role | Size | Weight | Line Height | Usage |
|------|------|--------|-------------|-------|
| Body | 14px (text-sm) | 400 (normal) | 1.43 (20px) | Timeline event descriptions, chat messages |
| Micro | 10px (text-[10px]) | 500 (medium) | 1.4 (14px) | Timeline badges, timestamps, duration labels, "Show data" toggle |
| Nano | 9px (text-[9px]) | 400 (normal) | 1.33 (12px) | Inter-event duration annotations |
| Label | 12px (text-xs) | 400 (normal) | 1.5 (18px) | Subtask descriptions, section headers, status bar items |
| Heading | 18px (text-lg) | 600 (semibold) | 1.33 (24px) | Task title in header |
| Section | 10px (text-[10px]) | 600 (semibold) | 1.4 (14px) | Uppercase tracking-wider section labels ("AGENTS (3)") |

---

## Color

The app runs in dark mode exclusively. All colors below reference the `.dark` CSS variables in `globals.css` plus hardcoded Tailwind utilities observed in the existing codebase.

| Role | Value | Usage |
|------|-------|-------|
| Dominant (60%) | `oklch(0.145 0 0)` / `bg-gray-950` | Page background, main surfaces |
| Secondary (30%) | `oklch(0.205 0 0)` / `bg-gray-900` | Cards, status bar (`bg-gray-900/50`), expanded data pre blocks (`bg-gray-900`) |
| Accent (10%) | `blue-500` / `blue-400` | Active tab underline, DAG running edge animation, agent dots, active view toggle |
| Destructive | `oklch(0.704 0.191 22.216)` / `red-400`/`red-500` | Failed status badges, cancel button, error messages |
| Border | `oklch(1 0 0 / 10%)` | All borders (`border-border`) |

### Timeline Category Colors (established in TraceTimeline.tsx -- do not change)

| Category | Dot Color | Badge Color |
|----------|-----------|-------------|
| task | `bg-blue-500` | `bg-blue-500/20 text-blue-400` |
| subtask | `bg-green-500` | `bg-green-500/20 text-green-400` |
| message | `bg-gray-500` | `bg-gray-500/20 text-gray-400` |
| approval | `bg-amber-500` | `bg-amber-500/20 text-amber-400` |
| policy | `bg-purple-500` | `bg-purple-500/20 text-purple-400` |
| template | `bg-cyan-500` | `bg-cyan-500/20 text-cyan-400` |
| agent | `bg-red-500` | `bg-red-500/20 text-red-400` |

### New: Replanning Event Color

| Category | Dot Color | Badge Color |
|----------|-----------|-------------|
| replan | `bg-amber-500` | `bg-amber-500/20 text-amber-400` |

Rationale: Replanning is a warning-level event (something failed, system is recovering). Amber matches the existing `approval` and `input_required` warning semantics. The `task.replanned` event type has prefix `task`, but its category should map to a new `replan` key in `categoryColors` to distinguish it from normal task lifecycle events.

Accent reserved for: active tab indicator (DAG/Timeline toggle), running status animations, agent identification dots, link hover states.

---

## Component Inventory: Phase 1 Changes

### 1. TraceTimeline.tsx -- Integration (OBSV-03)

**Status:** Component exists (untracked). Needs integration fixes, NOT a rewrite.

**Current integration point:** Task detail page (`web/app/tasks/[id]/page.tsx`, lines 300-304) already renders `<TraceTimeline taskId={taskId} />` inside the timeline view mode. The component is already wired.

**Required changes:**

| Change | Details |
|--------|---------|
| Add `task.replanned` to `describeEvent` switch | See Replanning section below |
| Add `replan` to `categoryColors` map | `{ dot: "bg-amber-500", badge: "bg-amber-500/20 text-amber-400" }` |
| Add `replan` mapping in `getCategory` | When `eventType === "task.replanned"`, return `"replan"` instead of `"task"` |
| Verify `api.tasks.timeline` returns `task.replanned` events | Backend already publishes them at `executor.go:986` |

**States:**

| State | Visual |
|-------|--------|
| Loading | Centered `Loader2` spinner with `animate-spin text-muted-foreground` (existing) |
| Error | Centered red-400 error text (existing) |
| Empty | "No events recorded for this task" in `text-muted-foreground` (existing) |
| Populated | Vertical timeline with dots, badges, descriptions, expandable data (existing) |

### 2. Replanning Event Visibility (OBSV-04)

**New `describeEvent` case for `task.replanned`:**

```
case "task.replanned":
  const replanData = event.data;
  const failedName = replanData.failed_subtask ?? "unknown";
  const newCount = replanData.new_subtask_count ?? 0;
  return `Replanned: subtask ${failedName} failed. ${newCount} new subtask${newCount !== 1 ? "s" : ""} created.`;
```

**Event data contract from backend** (`executor.go:986-990`):

| Field | Type | Description |
|-------|------|-------------|
| `replan_count` | number | How many times this task has been replanned (1-indexed) |
| `failed_subtask` | string | ID of the subtask that failed and triggered replan |
| `new_subtask_count` | number | Number of replacement subtasks created |

**Expanded data view:** The existing expand/collapse toggle already handles arbitrary `event.data` as JSON. No changes needed for the expandable section.

**Zustand store handler for `task.replanned`:**

Add to `handleEvent` in `web/lib/store.ts` after the `task.*` status mapping block (after line 169):

| Behavior | Implementation |
|----------|---------------|
| Reload subtasks from server | Call `selectTask(currentTask.id)` to refresh the full task (same pattern as `approval.resolved` at line 267) |
| Reason | Replanning deletes failed subtasks and creates new ones; a full reload is simpler and safer than trying to patch the subtask array from event data |

### 3. Bug Fixes (FOUND-01) -- Visual Contracts

Bug fixes do not introduce new UI components. Visual contracts for bug fix scope:

| Bug Category | Visual Contract |
|--------------|----------------|
| SSE race condition (FOUND-02) | No visual change. DAG nodes must transition from `running` to `completed`/`failed` without getting stuck. Test: after task completion, zero nodes should remain in `running` state. |
| Frozen DAG nodes | SubtaskNode status transitions must be immediate (<100ms from SSE event to visual update). No new UI needed -- fix is in event delivery order. |
| Any error states surfaced by fixes | Use existing `addToast("error", message)` pattern for transient errors. Use inline `text-red-400` for persistent error states. |

---

## Copywriting Contract

| Element | Copy |
|---------|------|
| Timeline tab label | "Timeline" (existing, line 258 of task detail page) |
| DAG tab label | "DAG" (existing, line 249) |
| Timeline empty state | "No events recorded for this task" (existing in TraceTimeline.tsx) |
| Timeline error state | "{error message from API}" (existing -- displays the caught error string) |
| Timeline loading | No text, spinner only (existing) |
| Replan event description | "Replanned: subtask {id} failed. {N} new subtask(s) created." |
| Expand toggle (collapsed) | "Show data" (existing) |
| Expand toggle (expanded) | "Hide data" (existing) |
| Cancel task button | "Cancel" / "Cancelling..." (existing) |
| Cancel success toast | "Task cancelled" (existing) |
| Cancel failure toast | "Failed to cancel task" (existing) |

**No new CTAs in Phase 1.** The timeline is read-only. No user-initiated actions are added beyond what already exists (DAG/Timeline tab toggle, expand/collapse data, cancel task).

---

## Interaction Contracts

### DAG/Timeline Tab Toggle

Already implemented at `web/app/tasks/[id]/page.tsx` lines 241-264. No changes needed.

| Property | Value |
|----------|-------|
| Active indicator | `border-b-2 border-blue-500 text-blue-400` |
| Inactive state | `text-muted-foreground hover:text-foreground` |
| Transition | `transition-colors` (Tailwind default 150ms) |
| Default view | `"dag"` (useState initial value) |

### Timeline Event Expand/Collapse

Already implemented in TraceTimeline.tsx lines 203-220. No changes needed.

| Property | Value |
|----------|-------|
| Trigger | Click on "Show data" / "Hide data" text button |
| Animation | None (instant toggle via conditional render) |
| Max height | `max-h-48` (192px) with `overflow-auto` |
| Data format | Pretty-printed JSON (`JSON.stringify(data, null, 2)`) |
| Font | Monospace (`pre` element), `text-[10px]` |

### SSE-Driven State Updates

| Event | Visual Response |
|-------|----------------|
| `task.replanned` | Timeline: new replan entry appears with amber dot/badge. DAG: full reload via `selectTask()` replaces subtask nodes. |
| `subtask.created` | DAG: new node appears. Timeline: new entry if timeline API is re-fetched. |
| `subtask.running` | DAG: node border changes to `border-blue-500`, spinner icon, edge animates. |
| `subtask.completed` | DAG: node border changes to `border-green-500`, checkmark icon. |
| `subtask.failed` | DAG: node border changes to `border-red-500`, X icon. |

**Timeline live updates:** The current TraceTimeline fetches events once on mount via `api.tasks.timeline(taskId)`. Phase 1 does NOT add live SSE-driven updates to the timeline view. If the user switches away from timeline tab and back, the component remounts and re-fetches. This is acceptable for Phase 1. Live timeline updates are a Phase 2+ enhancement.

---

## Registry Safety

| Registry | Blocks Used | Safety Gate |
|----------|-------------|-------------|
| shadcn official | button, input, card, badge, separator, scroll-area, toast, pagination | not required |
| Third-party | none | not applicable |

No third-party registries declared in `components.json` (`"registries": {}`). No new shadcn components needed for Phase 1.

---

## Checker Sign-Off

- [ ] Dimension 1 Copywriting: PASS
- [ ] Dimension 2 Visuals: PASS
- [ ] Dimension 3 Color: PASS
- [ ] Dimension 4 Typography: PASS
- [ ] Dimension 5 Spacing: PASS
- [ ] Dimension 6 Registry Safety: PASS

**Approval:** pending
