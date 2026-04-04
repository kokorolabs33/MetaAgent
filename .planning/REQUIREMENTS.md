# Requirements: TaskHub

**Defined:** 2026-04-04
**Core Value:** Developers can experience a complete A2A multi-agent collaboration flow in a polished, open-source package

## v1 Requirements

Requirements for GitHub open-source readiness. Each maps to roadmap phases.

### Foundation (GitHub Readiness)

- [ ] **FOUND-01**: Frontend-backend interaction bugs are fixed and all core flows work without errors
- [ ] **FOUND-02**: SSE subscribe/replay race condition is fixed — no dropped events or stuck DAG nodes
- [ ] **FOUND-03**: One-click local startup via `docker compose up` with seeded demo agents and pre-configured database
- [ ] **FOUND-04**: Comprehensive README with hero screenshot/GIF, badges, architecture diagram, and quick-start instructions

### Observability

- [ ] **OBSV-01**: Agent status indicators show online/working/idle/offline states with color-coded dots and pulse animation when working
- [ ] **OBSV-02**: Agent status changes are pushed via SSE on a global agent status channel (not polled)
- [ ] **OBSV-03**: Subtask timeline/trace view shows chronological execution with agent name, duration, and output per subtask
- [ ] **OBSV-04**: Adaptive replanning events are visible in the UI with "replanned because X" notifications and old vs new subtask comparison

### Interaction

- [ ] **INTR-01**: User can send messages to specific sub-agents during active task execution via @mention in chat (advisory mode)
- [ ] **INTR-02**: Multi-task parallel view dashboard shows multiple running tasks simultaneously with status badges and active agent indicators
- [ ] **INTR-03**: Multi-task SSE connection strategy handles browser connection limits (HTTP/2 or multiplexed endpoint)
- [ ] **INTR-04**: Dashboard supports task filtering by status (pending/running/completed/failed) and search by title

### Templates

- [ ] **TMPL-01**: User can save a successful task's orchestration pattern as a reusable template with variable parameterization
- [ ] **TMPL-02**: New tasks automatically match against existing templates and suggest relevant ones based on similarity
- [ ] **TMPL-03**: Template execution statistics are recorded and displayed (usage count, success rate, average duration)

## v2 Requirements

Deferred to future milestone. Tracked but not in current roadmap.

### Developer Experience

- **DX-01**: Mock LLM mode — system works without API key using canned responses for demo purposes
- **DX-02**: Token/cost display per task — show total tokens and estimated cost on task detail page

### A2A Showcase

- **A2A-01**: A2A Dev Panel — toggle-able panel showing raw JSON-RPC call/response for selected subtask
- **A2A-02**: Agent Card viewer — display agent-card.json for each registered agent

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| OpenTelemetry / external observability integration | LangFuse/LangSmith already do this; weeks of work for zero differentiation |
| Visual workflow builder (drag-and-drop DAG editor) | n8n has 150k+ stars on this; TaskHub's value is LLM-driven DAG creation |
| Multi-tenancy / org management | SaaS-tier feature; adds auth complexity that obscures A2A showcase |
| Plugin/extension system for custom agent types | Premature generalization; A2A protocol itself is the extension mechanism |
| Evaluation / LLM-as-judge scoring | Langfuse has full eval infrastructure; no demo payoff for TaskHub |
| Mobile-responsive UI | Desktop-first developer tool; don't break mobile but don't optimize |
| Agent marketplace / registry | Premature; needs community adoption first; agent-card.json is the discovery mechanism |
| Real-time multi-user collaboration | Complex WebSocket infrastructure for a feature that barely matters in single-developer showcase |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| FOUND-01 | Phase 1 | Pending |
| FOUND-02 | Phase 1 | Pending |
| FOUND-03 | Phase 1 | Pending |
| FOUND-04 | Phase 1 | Pending |
| OBSV-01 | Phase 2 | Pending |
| OBSV-02 | Phase 2 | Pending |
| OBSV-03 | Phase 1 | Pending |
| OBSV-04 | Phase 1 | Pending |
| INTR-01 | Phase 5 | Pending |
| INTR-02 | Phase 4 | Pending |
| INTR-03 | Phase 4 | Pending |
| INTR-04 | Phase 4 | Pending |
| TMPL-01 | Phase 3 | Pending |
| TMPL-02 | Phase 3 | Pending |
| TMPL-03 | Phase 3 | Pending |

**Coverage:**
- v1 requirements: 15 total
- Mapped to phases: 15
- Unmapped: 0

---
*Requirements defined: 2026-04-04*
*Last updated: 2026-04-04 after roadmap creation*
