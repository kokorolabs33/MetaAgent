---
phase: 05-chat-intervention
plan: 02
subsystem: frontend
tags: [advisory, typing-indicator, autocomplete, sse, zustand, chat-ui]

# Dependency graph
requires:
  - phase: 05-chat-intervention
    plan: 01
    provides: SendAdvisory backend pipeline, agent.typing SSE events, advisory_errors response field
  - phase: 02-agent-status
    provides: AgentStatusDot component, AgentActivityStatus type
provides:
  - TypingIndicator component for real-time agent processing feedback
  - Advisory reply badge rendering in ChatMessage
  - Status-enriched @mention autocomplete with inactive agent blocking
  - typingAgents Zustand state with SSE event handling
  - SendMessageResponse type guard for safe api response handling
affects: [frontend-chat, task-detail-page]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Client-side safety timeout (65s) for ephemeral SSE state to prevent stuck indicators"
    - "Discriminated union return type with type guard for API responses containing optional error data"
    - "Subtask-derived agent activity status with graceful fallback when subtasks are not yet loaded"

key-files:
  created:
    - web/components/chat/TypingIndicator.tsx
  modified:
    - web/components/chat/ChatMessage.tsx
    - web/components/chat/GroupChat.tsx
    - web/lib/store.ts
    - web/lib/api.ts

key-decisions:
  - "Used animate-bounce (not animate-pulse) for typing dots -- provides the classic bouncing dot pattern vs. the fade-in-out of animate-pulse used by AgentStatusDot for working state"
  - "agentsWithStatus falls back to isActive:true, hasSubtask:true when subtasks is undefined -- prevents empty autocomplete on first page load (Pitfall 7 from RESEARCH.md)"
  - "sendMessage returns string[] | undefined instead of void -- enables GroupChat to display advisory_errors inline without additional store state"
  - "Typing indicators placed between message feed and input area (not inside the feed) -- avoids scroll interference and stays visually anchored near the input"

patterns-established:
  - "isSendMessageResponse type guard: safe discriminated union handling for API responses that may contain additional error data alongside the primary resource"
  - "Client-side timeout cleanup for ephemeral SSE state: setTimeout auto-clear as safety net for lost events"

requirements-completed: [INTR-01]

# Metrics
duration: 6min
completed: 2026-04-07
---

# Phase 5 Plan 02: Advisory Chat Intervention Frontend Summary

**TypingIndicator with bouncing dots, advisory reply badge on ChatMessage, status-enriched autocomplete with AgentStatusDot and inactive blocking, SSE typing handlers with 65s safety timeout, SendMessageResponse type guard**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-07T00:42:47Z
- **Completed:** 2026-04-07T00:49:04Z
- **Tasks:** 2 completed + 1 checkpoint (human-verify)
- **Files created:** 1
- **Files modified:** 4

## Accomplishments

- Created TypingIndicator.tsx with staggered animate-bounce dots animation and agent name display
- Added isAdvisory metadata check and "Advisory reply" pill badge to ChatMessage (bg-blue-500/10 + text-blue-400)
- Exported SendMessageResponse interface and isSendMessageResponse type guard from api.ts
- Updated messages.send return type to Message | SendMessageResponse discriminated union
- Added typingAgents Record<string, string> to TaskStore with agent.typing and agent.typing_stopped SSE handlers
- Implemented 65-second client-side safety timeout for auto-clearing stuck typing indicators
- Added metadata field to SSE message event handler for advisory label rendering
- Updated sendMessage to return advisory_errors via type guard (no unsafe casts)
- Built agentsWithStatus useMemo deriving activity from subtask status, with fallback for undefined subtasks
- Enriched autocomplete dropdown with AgentStatusDot, "(not active)" label, and opacity-50 for inactive agents
- Blocked inactive agent selection with inline error message (D-02)
- Rendered TypingIndicator components between message feed and input area (D-07)
- Added advisory error display in red-tinted bar above input

## Task Commits

Each task was committed atomically:

1. **Task 1: Add TypingIndicator, advisory reply badge, SendMessageResponse type** - `7490138` (feat)
2. **Task 2: Enrich autocomplete with status, typing indicators, advisory error handling** - `0d335d2` (feat)

## Files Created/Modified

- `web/components/chat/TypingIndicator.tsx` - New component: bouncing dots animation with agentName prop, Tailwind animate-bounce with staggered delays
- `web/components/chat/ChatMessage.tsx` - Added isAdvisory useMemo checking message.metadata.advisory, renders "Advisory reply" rounded pill badge for agent advisory responses
- `web/components/chat/GroupChat.tsx` - Status-enriched autocomplete with AgentStatusDot and inactive blocking, TypingIndicator rendering from store, advisory error display, updated insertMention to block inactive agents
- `web/lib/store.ts` - Added typingAgents state, agent.typing/agent.typing_stopped SSE handlers with 65s safety timeout, metadata in message handler, sendMessage returns advisory_errors
- `web/lib/api.ts` - Added SendMessageResponse interface, isSendMessageResponse type guard, updated messages.send return type to discriminated union

## Decisions Made

- **animate-bounce vs animate-pulse:** Used animate-bounce for typing dots (classic bouncing pattern) rather than animate-pulse (which AgentStatusDot uses for working state). This provides visual distinction between "agent is active" (pulse) and "agent is typing" (bounce).
- **Subtask fallback strategy:** When subtasks is undefined or empty (before load), all agents show as isActive:true with hasSubtask:true. This prevents Pitfall 7 (empty autocomplete on first load) while still correctly filtering once subtask data arrives.
- **sendMessage return type:** Changed from Promise<void> to Promise<string[] | undefined> to propagate advisory_errors to GroupChat without adding separate store state. GroupChat handles display directly.
- **Typing indicator placement:** Rendered between message feed and input area (not inside the scrollable feed). This keeps indicators visually anchored near the input for clear feedback flow (D-12).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed store.ts type error from api.ts union type change**
- **Found during:** Task 1
- **Issue:** Changing api.ts messages.send return type to `Message | SendMessageResponse` caused TypeScript error in store.ts sendMessage which accessed `msg.id` directly on the union type
- **Fix:** Added isSendMessageResponse import and type guard usage in sendMessage (planned for Task 2 but needed for Task 1 typecheck to pass)
- **Files modified:** `web/lib/store.ts`
- **Commit:** 7490138 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Minor reordering -- store.ts type guard usage pulled forward from Task 2 to Task 1 to keep typecheck green between commits. No scope change.

## Threat Flags

No new security surfaces introduced beyond those documented in the plan's threat model. All SSE event data is typed before use with null checks. Advisory errors are rendered as plain text only. No HTML injection vectors.

## Known Stubs

None -- all components are fully wired to real data sources (SSE events for typing, metadata for advisory badge, subtask status for autocomplete).

## User Setup Required

None -- no external service configuration required.

## Self-Check: PASSED

All files exist, all commits verified (7490138, 0d335d2), tsc --noEmit passes, eslint passes, all artifact patterns confirmed.
