# Project Research Summary

**Project:** TaskHub — A2A Meta-Agent Platform
**Domain:** Open-source multi-agent orchestration platform (developer showcase)
**Researched:** 2026-04-04
**Confidence:** HIGH

## Executive Summary

TaskHub is an open-source A2A-protocol multi-agent orchestration platform targeting developers as its primary audience. The research confirms that the existing Go + Next.js + PostgreSQL + React Flow stack is sound and needs only minimal additions (shadcn Resizable for multi-task view, optionally @dnd-kit for template step reordering). All five milestone features — agent status visualization, enhanced chat interaction, multi-task parallel view, task templates with experience accumulation, and open-source readiness — can be delivered on the existing stack with no new backend dependencies and at most two new frontend packages. The A2A protocol (v1.0.0, Linux Foundation, March 2026) is well-served by the existing adapter layer; no SDK migration is needed.

The recommended build order follows a clear dependency chain. Agent status visualization comes first because it produces the global `agentStatusStore` and `/api/agents/stream` SSE endpoint that every other feature reuses. Template experience recording and open-source readiness are parallel tracks with no inter-dependencies. Multi-task parallel view builds on agent status components. Sub-agent direct chat is the most complex feature and should be scoped carefully — the research identifies a fundamental design decision (advisory vs. directive intervention model) that must be made before writing code to avoid a rewrite.

Three risks dominate. First, a subscribe-before-replay race in the SSE streaming layer can cause DAG nodes to freeze in "running" state — this must be fixed before adding more real-time status surfaces. Second, the multi-task parallel view will silently fail for 4+ concurrent tasks unless the team makes an explicit decision between HTTP/2 TLS and a multiplexed SSE endpoint. Third, the platform's current dependency on the `claude` CLI for LLM orchestration is a first-clone blocker for open-source adoption; a startup check and mock LLM mode are required before any public launch. Addressing these three in the first phase prevents compounding problems later.

---

## Key Findings

### Recommended Stack

The existing stack requires no structural changes. The project already carries all visualization, state management, styling, and real-time streaming capabilities needed for the milestone features. React Flow v12's built-in `NodeStatusIndicator` eliminates a dependency for agent status animations; Tailwind v4's `animate-ping` handles status pulse indicators. Zustand v5's built-in `persist` middleware covers template draft storage. The only genuinely new dependency is shadcn's `resizable` component (backed by `react-resizable-panels`) for the multi-task dashboard split-pane layout.

The A2A protocol v1.0.0 (released March 12, 2026, Linux Foundation, Apache 2.0) is directly supported by the existing adapter layer. The official Go SDK (`a2aproject/a2a-go`) is a convenience wrapper; replacing the custom adapter would add risk with no feature benefit this milestone.

**New dependencies (additions only):**
- `shadcn resizable` — multi-task dashboard panel layout — install via `pnpm dlx shadcn@latest add resizable`; let shadcn pin the react-resizable-panels version (v4 API breaking change noted, avoid direct install)
- `@dnd-kit/core` + `@dnd-kit/sortable` v6.x — template step reordering — conditional on drag-to-reorder being in scope; verify React 19 compatibility before adding
- `Zustand persist middleware` — template draft persistence — already bundled in Zustand v5.0.11

**Ruled out:** Framer Motion (bundle cost, Tailwind sufficient), react-beautiful-dnd (abandoned, React 19 incompatible), @tanstack/react-query (would require migrating all existing fetching), OpenTelemetry (weeks of work, zero differentiation), Next.js Parallel Routes for multi-task view (architectural overhead not justified).

### Expected Features

**Must have (table stakes) — unblocked by and unblocking all else:**
- Frontend bug fixes — broken UI on a demo repo signals unmaintained; unblocks everything
- One-click startup (Docker Compose + seeded demo agents) — the #1 open-source adoption factor
- Agent status indicators (online/idle/working/offline) — every agent platform shows this; absence is conspicuous
- README with hero demo GIF + quickstart — top agentic repos all have it; without it visitors bounce in 10s
- Task trace/timeline view — developers immediately look for "what did each agent do?"; `TraceTimeline.tsx` stub exists
- Cost/token display per task — LLM cost awareness is now hygiene; audit data already captured

**Should have (competitive differentiators):**
- Multi-task parallel view — no open-source A2A platform has this; high visual demo impact
- Adaptive replanning visibility — TaskHub already does replanning but it is invisible; mostly frontend work
- A2A protocol dev panel (show JSON-RPC wire calls) — makes TaskHub the canonical learning resource for A2A
- User-to-sub-agent direct chat during execution — no competitor does this; TaskHub's flagship differentiator

**Defer (v2+):**
- Template auto-match (part 2) — defer until templates are in active use
- Visual DAG builder — n8n has 150k+ stars; competing is a years-long effort
- OpenTelemetry / external observability integration — LangFuse already does this; link in README instead
- Multi-tenancy, plugin system, agent marketplace, mobile-responsive UI — all premature for a showcase

**Dependency chain:** Bug-free interaction → one-click startup → README/demo → GitHub launch. These three ship together as a "GitHub readiness" batch; none is useful without the others.

### Architecture Approach

The milestone extends the existing event-driven architecture without restructuring it. Four new components are added: (1) an `AgentActivityTracker` goroutine that derives working/idle status from live subtask counts and publishes to a new "agents" broker topic; (2) an `InterventionRouter` service inside the conversation handler that routes @mentions to the correct A2A agent contextId during active task execution; (3) a `parallelTaskStore` Zustand store managing N concurrent SSE connections for the multi-task dashboard; and (4) a `TemplateMatcher` that scores template suggestions via keyword overlap (no LLM) at task creation time. All four integrate through the existing broker/SSE/Zustand pattern — no new transport layer is introduced.

**Major components:**
1. **Agent Activity Status Layer** — derives four-state presence (online/idle/working/offline) from `agents.is_online` + live subtask count; broadcasts via new "agents" broker topic and `/api/agents/stream` SSE endpoint
2. **InterventionRouter** — thin service extending `ConversationHandler`; resolves @mentions to agent IDs, finds active subtask contextId, dispatches to A2A client; returns 202 immediately (existing fire-and-forget pattern)
3. **parallelTaskStore + TaskMonitorGrid** — Zustand store managing pinned task IDs, per-task SSE connections capped at 6, and live task state map; renders via `TaskMonitorCard` and `MiniDAG` components
4. **TemplateMatcher** — keyword overlap scoring (TF-IDF-style, < 5ms, zero cost) at task creation time; closes the execution recording loop in the executor for the experience accumulation feedback cycle

**Key patterns to follow:**
- Derive status from events; do not store derived state as a DB column (avoids dual-write problem)
- Single global agent status SSE channel (`/api/agents/stream`) — not per-agent polling
- Fan-out after persisting, not before — existing ordering must not be short-circuited
- Per-conversation SSE (not per-task SSE) for chat — existing `conversationStore` unification must be maintained
- HTTP handler returns 202 immediately for A2A dispatch; response via SSE (never block the handler)

### Critical Pitfalls

1. **SSE subscribe-before-replay race** (`internal/handlers/stream.go:53-74`) — subscribe to broker first, replay DB events second, deduplicate by event ID; must fix before adding more status surfaces or DAG nodes will freeze in "running" state
2. **Multi-task view exhausts HTTP/1.1 SSE connection limit at 4+ tasks** — decide between HTTP/2 TLS (cleaner) or multiplexed `/api/events/stream?tasks=id1,id2` endpoint before building the feature; silently breaks in demos
3. **Stale "online" indicator (up to 2-minute gap)** — add staleness threshold: if `now - last_health_check > threshold`, render as "unknown"; proactively mark agent offline when a subtask fails with unreachable error
4. **Chat intervention race with executor** (`executor.go` vs `conversations.go`, no shared mutex) — decide advisory vs. directive model before writing code; advisory is safe with current architecture; directive requires adding an `input_queue` channel to subtask context
5. **Template captures specific agent UUIDs and hard context, not reusable structure** — design variable extraction (`{{project_name}}` placeholders) before building the `StepEditor` UI; retrofitting requires schema migration

---

## Implications for Roadmap

Based on combined research, the following phase structure is recommended. The ordering is driven by three forces: (a) the "GitHub readiness" dependency chain, (b) architectural dependencies between new components, and (c) the rule that pitfalls flagged as "must resolve before building" become phase preconditions.

### Phase 0: GitHub Readiness + Foundation Fixes

**Rationale:** Nothing else matters if the repo fails on first clone or has broken UI. This is the precondition for every other phase having demo value. Three of the thirteen pitfalls (SSE race, stale health indicator, Claude CLI dependency) must be patched here to prevent them from compounding across later phases.

**Delivers:** Repo that a developer can clone, `docker compose up`, and see a working A2A orchestration in under 2 minutes

**Addresses:** One-click startup, README/demo GIF, frontend bug fixes, cost/token display (data already exists), task trace/timeline view (`TraceTimeline.tsx` stub exists)

**Must avoid:**
- Pitfall 9 (Claude CLI first-clone blocker) — add `--mock-llm` mode and startup check
- Pitfall 1 (SSE subscribe-before-replay race) — fix before adding more event surfaces
- Pitfall 3 (stale health indicator) — add staleness threshold to API response

**Research flag:** Standard patterns; no deeper phase research needed. Docker Compose, README best practices, and SSE dedup are all well-documented.

---

### Phase 1: Agent Status Visualization

**Rationale:** This phase produces shared infrastructure (`agentStatusStore`, `/api/agents/stream`, `AgentStatusDot`) that Phases 2 and 3 depend on directly. It also has the lowest risk of the four feature phases. Build it first, get the global status channel stable, then build on top of it.

**Delivers:** Real-time online/idle/working/offline indicators on agent list, agent detail, and DAG nodes; global agent SSE channel serving all subsequent features

**Addresses:** Agent status indicators (table stakes); `NodeStatusIndicator` from React Flow v12 (zero new dependency)

**Implements:** Agent Activity Status Layer — extend broker with "agents" topic, add `AgentActivityTracker`, add `/api/agents/stream` SSE endpoint, add `agentStatusStore.ts` and `AgentStatusDot.tsx`

**Must avoid:**
- Pitfall 12 (undefined SSE event types) — define `agent.working`, `agent.idle`, `agent.offline` in `types.ts` before backend emitters
- Pitfall 7 (broker channel drop under load) — increase buffer or add drop counter with forced reconnect

**Research flag:** Well-documented patterns (SSE fanout, Zustand store, React Flow NodeStatusIndicator). No deeper research needed.

---

### Phase 2: Task Templates + Experience Accumulation

**Rationale:** Parallel track to Phase 1 with no dependency on agent status infrastructure. Fixes an existing data gap (executor does not record `template_executions`) and adds template suggestion to task creation. Medium complexity, high strategic value for the "learning loop" differentiator.

**Delivers:** "Save as template" from completed task, keyword-based template suggestion at task creation, execution history recording, evolution suggestions panel

**Addresses:** Task templates (differentiator — part 1: save + suggest); defers part 2 (auto-match improvement) to v2

**Implements:** TemplateMatcher, executor extension for `template_execution` recording, `TemplateSuggestionBar.tsx`, `TemplateEvolutionPanel.tsx`

**Must avoid:**
- Pitfall 5 (template captures specific context) — design `{{variable}}` placeholder extraction at template creation time before building StepEditor UI
- Pitfall 8 (silent template execution tracking failure) — fix `_ =` error discards in executor template recording paths; add integration test
- Pitfall 11 (circular dependency in step editor) — add Kahn's algorithm cycle detection in template save handler
- Pitfall 13 (prompt injection via template variables) — add max-length limits and input sanitization before variables reach the LLM prompt

**Research flag:** Template variable extraction design may need a short research spike on the right parameterization UX pattern (placeholders vs. prompted variables). All other patterns are standard Go/PostgreSQL.

---

### Phase 3: Multi-Task Parallel View

**Rationale:** Depends on Phase 1 (`AgentStatusDot` used in `TaskMonitorCard`, `agentStatusStore` shared). Primarily frontend work — no new backend endpoints. High visual demo impact and direct differentiation from competitors. The SSE connection limit pitfall is an architecture decision that gates the entire feature.

**Delivers:** Dashboard grid showing N active tasks with live mini-DAG, status badge, agent dots, and latest message; pin/unpin controls; click-through to full conversation view

**Addresses:** Multi-task parallel view (differentiator); task filtering/search (can ship as simple filter buttons alongside)

**Implements:** `parallelTaskStore.ts`, `MiniDAG.tsx`, `TaskMonitorCard.tsx`, `TaskMonitorGrid.tsx`; minor backend: `GET /api/tasks?status=running` filter

**Must avoid:**
- Pitfall 2 (HTTP/1.1 6-connection limit) — decide HTTP/2 TLS vs. multiplexed SSE endpoint as Phase 3 kickoff decision; do not start building without resolving this
- Pitfall 6 (stale Zustand closure) — restructure store to `Map<taskId, TaskViewState>` before adding second task pane

**Research flag:** The HTTP/2 vs. multiplexed SSE decision benefits from a quick technical spike if the deployment target does not already have TLS configured.

---

### Phase 4: Enhanced Chat Interaction (Sub-Agent Intervention)

**Rationale:** Most complex feature; builds on all prior phases. Depends on Phase 1 for visual feedback when an agent is responding. The advisory/directive intervention model decision is a hard precondition — the wrong choice leads to a full rewrite of `InterventionRouter`. Build last to avoid changing conversation handler plumbing multiple times.

**Delivers:** Users can send @mention messages to running sub-agents mid-execution; `input_required` subtask state unblocked by user chat message; agent replies appear inline with correct sender attribution

**Addresses:** User-to-sub-agent direct chat (flagship differentiator)

**Implements:** `InterventionRouter` in `handlers/conversations.go`, A2A contextId threading for active subtasks, `POST /api/tasks/{id}/subtasks/{subtask_id}/input` endpoint, `input_required` state in `ConversationView`

**Must avoid:**
- Pitfall 4 (intervention race with executor) — define advisory vs. directive model explicitly; start with advisory (no executor coordination needed) and add directive as a follow-on
- Pitfall 10 (unbounded message history) — add cursor-based pagination before building the richer chat UI

**Research flag:** This phase needs deeper research or a design spike on A2A `input_required` state mechanics and contextId threading before implementation begins. The A2A spec's task state machine (WORKING → INPUT_REQUIRED → WORKING → COMPLETED) must be implemented correctly.

---

### Phase 5: A2A Showcase + Adaptive Replanning Visibility

**Rationale:** Lower complexity; builds on stable foundation from Phases 0-3. Surfaces existing backend capabilities (replanning events, A2A wire data) that are already there but invisible. Directly serves the "canonical A2A learning resource" positioning.

**Delivers:** Dev panel showing agent-card.json and raw A2A JSON-RPC call/response for selected subtask; `task.replanned` event surfaced as special timeline entry with old vs. new subtask diff

**Addresses:** A2A dev panel (differentiator), adaptive replanning visibility (differentiator)

**Research flag:** No new backend work; purely frontend composition of existing audit/event data. Standard patterns; no research phase needed.

---

### Phase Ordering Rationale

- Phase 0 before everything — broken demo means no GitHub traction; SSE race fix prevents it from being baked into all new features
- Phase 1 before Phases 3 and 4 — `agentStatusStore` is shared infrastructure; building multi-task view or intervention UI without it means duplicating status logic
- Phase 2 is parallel to Phase 1 — template system touches different backend subsystems (executor, template handlers) with no SSE dependency
- Phase 3 after Phase 1 — `AgentStatusDot` directly used in `TaskMonitorCard`; clean dependency
- Phase 4 last — most complex, touches the most-coupled handler (`conversations.go`), needs all prior stabilization
- Phase 5 last — purely additive frontend; no risk to other features; good milestone closer

### Research Flags

Phases needing deeper research or design spikes before implementation:

- **Phase 3:** HTTP/2 vs. multiplexed SSE architecture decision — must be made before building; 1-2 hour technical spike recommended
- **Phase 4:** A2A `input_required` state mechanics and contextId threading — needs spec review and design doc before writing `InterventionRouter`; advisory vs. directive model must be documented as an explicit decision

Phases with well-documented patterns (no research phase needed):
- **Phase 0:** Docker Compose, README patterns, SSE dedup — all standard
- **Phase 1:** React Flow NodeStatusIndicator, Zustand store, SSE fanout — all documented
- **Phase 2:** Go keyword scoring, PostgreSQL JSONB, Zustand persist — all verified against existing codebase
- **Phase 5:** Frontend composition of existing data — no new patterns

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Verified against existing codebase, official docs, and npm. Version compatibility warning on react-resizable-panels v4 is documented with specific GitHub issue reference |
| Features | HIGH | Cross-referenced against 5+ competitor platforms; existing codebase verified for baseline capabilities |
| Architecture | HIGH | Primarily based on direct codebase reading (`HIGH confidence` per researcher). Workflow memory retrieval source returned 404 (one LOW confidence finding); does not affect core architecture |
| Pitfalls | HIGH | All critical pitfalls grounded in specific file/line references from codebase analysis and `CONCERNS.md`. External sources corroborate patterns |

**Overall confidence:** HIGH

### Gaps to Address

- **@dnd-kit/core React 19 compatibility** — not explicitly verified; check before adding to `package.json`. Workaround: implement basic drag using native HTML5 drag events if compatibility is unconfirmed
- **react-resizable-panels v4 + shadcn resizable compatibility** — GitHub issue #9136 was open as of research date; validate `pnpm dlx shadcn@latest add resizable` installs without error before building Phase 3 layout
- **Advisory vs. directive chat intervention model** — research surfaced the architectural fork but deliberately left the decision to the planning phase; this is a product decision (UX tradeoff: advisory is simpler and honest; directive is the "flagship" demo but requires executor coordination)
- **Template variable UX pattern** — placeholder syntax (`{{var}}`) vs. prompted variables at creation time vs. LLM-derived substitution; the right choice depends on target user sophistication; needs brief design decision before Phase 2 `StepEditor` work begins
- **HTTP/2 TLS for development deployment** — if `docker compose` serves over plain HTTP, the HTTP/2 SSE multiplexing benefit is unavailable (HTTP/2 requires TLS in browsers); multiplexed application-layer SSE endpoint is the safer default for dev tooling

---

## Sources

### Primary (HIGH confidence)
- Existing codebase: `internal/events/broker.go`, `internal/handlers/stream.go`, `internal/handlers/templates.go`, `internal/executor/executor.go`, `web/lib/store.ts`, `web/lib/sse.ts`, `internal/models/agent.go`, `internal/models/template.go`
- React Flow v12 NodeStatusIndicator: reactflow.dev/ui/components/node-status-indicator
- A2A Protocol v1.0.0: github.com/a2aproject/A2A + a2a-protocol.org/latest/specification
- Azure Architecture Center AI Agent Design Patterns: learn.microsoft.com/azure/architecture/ai-ml/guide/ai-agent-design-patterns (updated 2026-03-07)
- Tailwind CSS v4 animate-ping: tailwindcss.com docs
- Zustand v5 persist middleware: zustand.docs.pmnd.rs/reference/middlewares/persist
- shadcn/ui Tailwind v4 compatibility: ui.shadcn.com/docs/tailwind-v4

### Secondary (MEDIUM confidence)
- A2A Protocol specification task state enumeration — spec authoritative but partial content retrieved
- Real-time presence platform patterns (heartbeat, single fanout) — systemdesign.one, aligns with existing broker implementation
- LangGraph human-in-the-loop interrupt pattern (Python-specific, pattern applies to Go)
- Go health aggregation patterns — oneuptime.com/blog 2026-02-01
- shadcn resizable v4 compatibility issue — github.com/shadcn-ui/ui/issues/9136
- Browser SSE 6-connection limit (Chrome Won't Fix): issues.chromium.org/issues/40329530
- SSE HTTP/2 multiplexing solution: medium.com/@kaitmore
- Multi-agent failure modes: medium.com/@rakesh.sheshadri44
- LLM prompt injection via templates (OWASP LLM01:2025): genai.owasp.org/llmrisk/llm01-prompt-injection

### Tertiary (LOW confidence)
- Workflow memory and template retrieval (emergence.ai blog) — URL returned 404; finding based on search snippet only; does not affect core recommendations

---

*Research completed: 2026-04-04*
*Ready for roadmap: yes*
