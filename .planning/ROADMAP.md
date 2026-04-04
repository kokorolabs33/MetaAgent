# Roadmap: TaskHub

## Overview

TaskHub ships in five phases that transform a functional but rough codebase into a polished, demo-ready open-source A2A platform. Phase 1 fixes the foundation and ships GitHub-ready artifacts so the repo is credible on first clone. Phase 2 builds the shared agent status infrastructure that later phases depend on. Phase 3 delivers task templates in parallel — it shares no dependencies with Phase 2 and can proceed independently. Phase 4 adds the multi-task parallel dashboard, consuming the status infrastructure from Phase 2. Phase 5 completes the platform with the flagship sub-agent chat intervention feature, built last because it is the most complex and benefits from all prior stabilization.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Foundation** - Fix all interaction bugs, SSE race, and ship GitHub-ready repo artifacts
- [ ] **Phase 2: Agent Status** - Real-time online/working/idle/offline indicators with global SSE channel
- [ ] **Phase 3: Templates** - Save orchestration patterns as reusable templates with experience accumulation
- [ ] **Phase 4: Parallel Dashboard** - Multi-task parallel view with live DAG, status badges, and task filtering
- [ ] **Phase 5: Chat Intervention** - User can message sub-agents mid-execution via @mention in chat

## Phase Details

### Phase 1: Foundation
**Goal**: A developer can clone the repo, run `docker compose up`, and see a fully working A2A orchestration demo with no broken interactions and no frozen DAG nodes
**Depends on**: Nothing (first phase)
**Requirements**: FOUND-01, FOUND-02, FOUND-03, FOUND-04, OBSV-03, OBSV-04
**Success Criteria** (what must be TRUE):
  1. All core flows (task creation, DAG execution, agent messaging, task completion) complete without frontend errors or frozen states
  2. SSE stream delivers all events after page load — no DAG nodes stuck in "running" after task completion
  3. `docker compose up` starts a fully functional TaskHub with seeded demo agents in under 2 minutes, no manual configuration required
  4. README contains hero screenshot or GIF, badge row, architecture diagram, and a quickstart that works on first attempt
  5. Subtask trace/timeline view shows chronological execution with agent name, duration, and output per subtask; adaptive replanning events surface as "replanned because X" entries
**Plans:** 4 plans

Plans:
- [ ] 01-01-PLAN.md — SSE race fix + Anthropic SDK replacement (backend core)
- [ ] 01-02-PLAN.md — TraceTimeline replanning visibility (frontend)
- [ ] 01-03-PLAN.md — Docker Compose one-click startup + agent seeding
- [ ] 01-04-PLAN.md — README rewrite + demo verification checkpoint

**UI hint**: yes

### Phase 2: Agent Status
**Goal**: Every agent surface in the UI shows real-time online/working/idle/offline state, and the global agent status SSE channel is stable for downstream phases to consume
**Depends on**: Phase 1
**Requirements**: OBSV-01, OBSV-02
**Success Criteria** (what must be TRUE):
  1. Agent list and agent detail pages show color-coded status dots with pulse animation when an agent is actively working
  2. DAG nodes display agent status inline without requiring a page refresh
  3. Status changes arrive via SSE push — no polling — and update within 2 seconds of the underlying event
  4. Stale agents (last health check older than threshold) render as "unknown" rather than falsely "online"
**Plans**: TBD
**UI hint**: yes

### Phase 3: Templates
**Goal**: Users can save successful task orchestration patterns as reusable templates and receive suggestions when creating new tasks similar to previous ones
**Depends on**: Phase 1
**Requirements**: TMPL-01, TMPL-02, TMPL-03
**Success Criteria** (what must be TRUE):
  1. User can save a completed task as a template, with variable placeholders extracted for context-specific fields
  2. Task creation flow suggests matching templates based on title similarity before the user submits
  3. Each template's usage count, success rate, and average duration are visible on the template detail view
**Plans**: TBD
**UI hint**: yes

### Phase 4: Parallel Dashboard
**Goal**: Users can monitor multiple running tasks simultaneously from a single dashboard view with live status and one-click navigation to full task detail
**Depends on**: Phase 2
**Requirements**: INTR-02, INTR-03, INTR-04
**Success Criteria** (what must be TRUE):
  1. Dashboard shows a grid of running tasks, each with a live mini-DAG, status badge, and active agent dots
  2. User can filter tasks by status (pending/running/completed/failed) and search by title
  3. Four or more concurrent tasks display correctly without browser connection limit errors
  4. Clicking any task card navigates to the full task detail and conversation view
**Plans**: TBD
**UI hint**: yes

### Phase 5: Chat Intervention
**Goal**: Users can send advisory messages to specific sub-agents during active task execution, with responses appearing inline in the conversation stream
**Depends on**: Phase 2
**Requirements**: INTR-01
**Success Criteria** (what must be TRUE):
  1. User can @mention a specific sub-agent in the chat input while a task is running and the message is delivered to that agent's active context
  2. The agent's response to the intervention appears in the conversation with correct sender attribution
  3. The intervention does not block or crash the executor — the task continues executing regardless of intervention timing
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 0/4 | Planning complete | - |
| 2. Agent Status | 0/TBD | Not started | - |
| 3. Templates | 0/TBD | Not started | - |
| 4. Parallel Dashboard | 0/TBD | Not started | - |
| 5. Chat Intervention | 0/TBD | Not started | - |
