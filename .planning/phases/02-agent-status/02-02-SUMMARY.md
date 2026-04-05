---
phase: 02-agent-status
plan: 02
subsystem: ui
tags: [react, zustand, sse, agent-status, real-time, tailwind, shadcn]

# Dependency graph
requires:
  - phase: 02-agent-status plan 01
    provides: "/api/agents/stream SSE endpoint with agent.status_changed events"
provides:
  - "AgentStatusDot reusable component with 5 states (online/working/idle/offline/unknown)"
  - "useAgentStore agentStatuses map driven by SSE events"
  - "connectAgentStatusSSE helper in sse.ts"
  - "AgentActivityStatus type in types.ts"
  - "Real-time status dots on agent cards, agent detail, health overview, and DAG nodes"
affects: [04-parallel-dashboard, 05-chat-intervention]

# Tech tracking
tech-stack:
  added: []
  patterns: ["SSE-backed Zustand store for real-time UI state", "idempotent SSE connect/disconnect in useEffect lifecycle", "agentId pass-through in DAG node data for store lookup"]

key-files:
  created:
    - "web/components/agent/AgentStatusDot.tsx"
  modified:
    - "web/lib/types.ts"
    - "web/lib/sse.ts"
    - "web/lib/store.ts"
    - "web/components/agent/AgentCard.tsx"
    - "web/app/agents/[id]/page.tsx"
    - "web/app/agents/health/page.tsx"
    - "web/app/agents/page.tsx"
    - "web/app/tasks/[id]/page.tsx"
    - "web/components/task/SubtaskNode.tsx"
    - "web/components/task/DAGView.tsx"

key-decisions:
  - "AgentStatusDot uses Tailwind animate-pulse for working state and border-dashed for unknown state -- no custom CSS needed"
  - "SSE connection is initialized per page (agents list, health, task detail) via useEffect with cleanup -- idempotent via disconnect-before-connect"
  - "agentId added to SubtaskNodeData interface to enable status lookup from Zustand store in DAG nodes"
  - "Health page falls back to REST-loaded is_online when no SSE status has arrived yet"

patterns-established:
  - "SSE-backed Zustand store: connectAgentStatusSSE feeds agentStatuses map, components read via selector"
  - "Page-level SSE lifecycle: useEffect with connectStatusSSE/disconnectStatusSSE on mount/unmount"
  - "DAG node data enrichment: pass entity IDs (agentId) alongside display names for store lookups"

requirements-completed: [OBSV-01]

# Metrics
duration: 8min
completed: 2026-04-05
---

# Phase 2 Plan 2: Frontend Agent Status Indicators Summary

**Real-time AgentStatusDot component wired to SSE-backed Zustand store across all agent surfaces (cards, detail, health, DAG nodes)**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-05T03:43:00Z
- **Completed:** 2026-04-05T03:51:43Z
- **Tasks:** 3 (2 auto + 1 checkpoint)
- **Files modified:** 11

## Accomplishments
- Created AgentStatusDot component with 5 visual states: green (online/idle), amber+pulse (working), gray (offline), gray-dashed (unknown)
- Extended useAgentStore with agentStatuses map, connectStatusSSE/disconnectStatusSSE lifecycle, and getAgentStatus helper
- Added connectAgentStatusSSE helper in sse.ts following existing connectSSE pattern
- Placed status dots on all 4 agent UI surfaces: agent list cards, agent detail page, health overview table, and DAG subtask nodes
- Added agentId to SubtaskNodeData and DAGView node data construction for status lookup
- Initialized SSE connection on agents page, health page, AND task detail page so DAG nodes receive live updates

## Task Commits

Each task was committed atomically:

1. **Task 1: Create AgentStatusDot component + add types + SSE helper** - `6afa8e6` (feat)
2. **Task 2: Extend useAgentStore with status tracking + SSE, place on all surfaces** - `bab3b94` (feat)
3. **Task 3: Visual verification checkpoint** - auto-approved by user

## Files Created/Modified
- `web/components/agent/AgentStatusDot.tsx` - New reusable status dot component with 5 states, size variants, pulse animation, dashed border
- `web/lib/types.ts` - Added AgentActivityStatus type ("online" | "working" | "idle" | "offline" | "unknown")
- `web/lib/sse.ts` - Added connectAgentStatusSSE helper connecting to /api/agents/stream
- `web/lib/store.ts` - Extended useAgentStore with agentStatuses map, SSE connect/disconnect, getAgentStatus
- `web/components/agent/AgentCard.tsx` - Added AgentStatusDot inline with agent name in card header
- `web/app/agents/[id]/page.tsx` - Added AgentStatusDot to agent detail page title
- `web/app/agents/health/page.tsx` - Replaced static online/offline dot with SSE-driven AgentStatusDot
- `web/app/agents/page.tsx` - Added SSE connection lifecycle in useEffect
- `web/app/tasks/[id]/page.tsx` - Added SSE connection lifecycle for DAG node status dots
- `web/components/task/SubtaskNode.tsx` - Added agentId to interface, AgentStatusDot inline with agent name
- `web/components/task/DAGView.tsx` - Passes agentId in node data for SubtaskNode status lookup

## Decisions Made
- AgentStatusDot uses Tailwind's built-in animate-pulse class for working state animation -- no custom keyframes needed
- Unknown state uses `border border-dashed border-gray-500` instead of background color to visually distinguish from offline (solid gray)
- SSE connection managed per-page rather than globally to avoid keeping connections open on pages that don't render agent status
- Health page falls back to REST `is_online` field when no SSE event has arrived yet, providing correct initial state
- agentId added to SubtaskNodeData (was previously absent) to allow SubtaskNode to look up live status from Zustand store

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Known Stubs

None - all components are wired to the SSE-backed Zustand store with no placeholder data.

## Next Phase Readiness
- Phase 02 (agent-status) is now fully complete: backend SSE infrastructure (Plan 01) + frontend status indicators (Plan 02)
- All agent surfaces show real-time status dots driven by SSE events from /api/agents/stream
- Phase 04 (parallel-dashboard) can build on the established SSE-backed Zustand store pattern
- Phase 05 (chat-intervention) has a live agent activity signal available via agentStatuses in the store

---
*Phase: 02-agent-status*
*Completed: 2026-04-05*

## Self-Check: PASSED

All 11 source files verified present. Both task commits (6afa8e6, bab3b94) verified in git log.
