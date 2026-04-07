# Roadmap: TaskHub

## Milestones

- ✅ **v1.0 Meta-Agent Foundation** - Phases 1-6 (shipped 2026-04-07)
- 🚧 **v2.0 Wow Moment** - Phases 7-10 (in progress)

## Phases

<details>
<summary>v1.0 Meta-Agent Foundation (Phases 1-6) - SHIPPED 2026-04-07</summary>

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
- [x] 01-01-PLAN.md — SSE race fix + Anthropic SDK replacement (backend core)
- [x] 01-02-PLAN.md — TraceTimeline replanning visibility (frontend)
- [x] 01-03-PLAN.md — Docker Compose one-click startup + agent seeding
- [x] 01-04-PLAN.md — README rewrite + demo verification checkpoint

### Phase 2: Agent Status
**Goal**: Every agent surface in the UI shows real-time online/working/idle/offline state, and the global agent status SSE channel is stable for downstream phases to consume
**Depends on**: Phase 1
**Requirements**: OBSV-01, OBSV-02
**Success Criteria** (what must be TRUE):
  1. Agent list and agent detail pages show color-coded status dots with pulse animation when an agent is actively working
  2. DAG nodes display agent status inline without requiring a page refresh
  3. Status changes arrive via SSE push — no polling — and update within 2 seconds of the underlying event
  4. Stale agents (last health check older than threshold) render as "unknown" rather than falsely "online"
**Plans:** 2 plans

Plans:
- [x] 02-01-PLAN.md — Backend agent status infrastructure (executor tracking, HealthChecker Broker, SSE endpoint)
- [x] 02-02-PLAN.md — Frontend AgentStatusDot component + store + SSE wiring across all surfaces

### Phase 3: Templates
**Goal**: Users can save successful task orchestration patterns as reusable templates and receive suggestions when creating new tasks similar to previous ones
**Depends on**: Phase 1
**Requirements**: TMPL-01, TMPL-02, TMPL-03
**Success Criteria** (what must be TRUE):
  1. User can save a completed task as a template, with variable placeholders extracted for context-specific fields
  2. Task creation flow suggests matching templates based on title similarity before the user submits
  3. Each template's usage count, success rate, and average duration are visible on the template detail view
**Plans**: 3 plans

Plans:
- [x] 03-01-PLAN.md — Template backend + API
- [x] 03-02-PLAN.md — Template frontend
- [x] 03-03-PLAN.md — Template suggestions

### Phase 4: Parallel Dashboard
**Goal**: Users can monitor multiple running tasks simultaneously from a single dashboard view with live status and one-click navigation to full task detail
**Depends on**: Phase 2
**Requirements**: INTR-02, INTR-03, INTR-04
**Success Criteria** (what must be TRUE):
  1. Dashboard shows a grid of running tasks, each with a live mini-DAG, status badge, and active agent dots
  2. User can filter tasks by status (pending/running/completed/failed) and search by title
  3. Four or more concurrent tasks display correctly without browser connection limit errors
  4. Clicking any task card navigates to the full task detail and conversation view
**Plans:** 3 plans

Plans:
- [x] 04-01-PLAN.md — Backend: extended task list with subtask counts + multiplexed SSE endpoint
- [x] 04-02-PLAN.md — Frontend components: progress bar, dashboard card, empty states, filter bar
- [x] 04-03-PLAN.md — Integration: SSE helper + dashboard store + page wiring + human verification

### Phase 5: Chat Intervention
**Goal**: Users can send advisory messages to specific sub-agents during active task execution, with responses appearing inline in the conversation stream
**Depends on**: Phase 2
**Requirements**: INTR-01
**Success Criteria** (what must be TRUE):
  1. User can @mention a specific sub-agent in the chat input while a task is running and the message is delivered to that agent's active context
  2. The agent's response to the intervention appears in the conversation with correct sender attribution
  3. The intervention does not block or crash the executor — the task continues executing regardless of intervention timing
**Plans:** 2 plans

Plans:
- [x] 05-01-PLAN.md — Backend: SendAdvisory method + routeAdvisory validation + typing indicator SSE events
- [x] 05-02-PLAN.md — Frontend: advisory reply label + TypingIndicator + status-enriched autocomplete + human verification

### Phase 6: Demo Readiness
**Goal**: All manage pages are functional with real or seeded data, the platform can run with OpenAI key for task decomposition, and every page is demo-worthy
**Depends on**: Phase 4, Phase 5
**Requirements**: DEMO-01, DEMO-02, DEMO-03, DEMO-04, DEMO-05, DEMO-06
**Success Criteria** (what must be TRUE):
  1. Task decomposition works with OpenAI API key (not just Anthropic)
  2. /manage/templates shows seeded template data with usage stats
  3. /manage/analytics has per-agent drill-down with task assignments and time/status filters
  4. /manage/audit shows audit logs with filtering by time, agent, and event type
  5. /manage/settings/policies shows seeded policy data
**Plans:** 4 plans

Plans:
- [x] 06-01-PLAN.md — OpenAI LLM client extraction + orchestrator replacement
- [x] 06-02-PLAN.md — Seed templates, policies, and demo task data
- [x] 06-03-PLAN.md — Analytics filters + per-agent drill-down
- [x] 06-04-PLAN.md — Audit time range filter + template usage stats

</details>

### v2.0 Wow Moment (In Progress)

**Milestone Goal:** Make agents do real work and produce visible results — transform the demo from "agents chatting" to "agents working with real data and producing actionable outputs"

- [ ] **Phase 7: Agent Tool Use** - Function calling with web search so agents retrieve real data instead of hallucinating
- [ ] **Phase 8: Artifact Rendering** - Rich typed cards for search results, code, and tables instead of raw text blobs
- [ ] **Phase 9: Streaming Output** - Token-by-token agent replies so users watch agents think in real time
- [ ] **Phase 10: Inbound Webhooks** - External events from GitHub and Slack trigger task creation automatically

## Phase Details

### Phase 7: Agent Tool Use
**Goal**: Agents can call tools during task execution — starting with web search — and users see tool activity in real time in the chat feed
**Depends on**: Phase 6 (v1.0 complete)
**Requirements**: TOOL-01, TOOL-02, TOOL-03, TOOL-04
**Success Criteria** (what must be TRUE):
  1. An agent asked to research a current topic calls web search and returns results grounded in real-time data (not hallucinated from training knowledge)
  2. The chat feed shows tool call events as they happen — user sees "Searching for: [query]..." before the agent's final response arrives
  3. Different agent roles have different tool sets visible in their responses (e.g., Engineering agent can analyze code, Marketing agent searches the web)
  4. Multi-turn tool use works correctly — an agent can call a tool, process results, call another tool, and produce a final response without conversation corruption
**Plans**: 2 plans

Plans:
- [x] 07-01-PLAN.md — Tool registry, ChatWithTools method, Tavily web search, per-role tool sets
- [x] 07-02-PLAN.md — SSE tool call events, frontend inline status component, end-to-end verification

### Phase 8: Artifact Rendering
**Goal**: Structured agent outputs render as rich, interactive UI cards instead of raw text — making tool results and agent work visually compelling
**Depends on**: Phase 7
**Requirements**: ARTF-01, ARTF-02, ARTF-03, ARTF-04
**Success Criteria** (what must be TRUE):
  1. Search results from web search tools render as clickable source cards with title, URL, and snippet — not as raw JSON or plain text
  2. Code blocks in agent output have syntax highlighting with language detection, and tables render as formatted HTML tables (not ASCII)
  3. Users can copy artifact content to clipboard or download it as a file with one click
  4. All markdown in agent messages renders with GFM support (tables, task lists, strikethrough) and syntax-highlighted code blocks
**Plans**: 2 plans
**UI hint**: yes

Plans:
- [x] 08-01-PLAN.md — Artifact type contracts, markdown upgrade (react-markdown + remark-gfm + rehype-highlight), executor metadata pipeline
- [x] 08-02-PLAN.md — Rich artifact card components (SearchResultsCard, CodeBlock, TableCard, DataCard), copy/download actions, end-to-end verification

### Phase 9: Streaming Output
**Goal**: Agent replies stream to the browser token-by-token so users watch agents think and write in real time instead of waiting for complete responses
**Depends on**: Phase 7
**Requirements**: STRM-01, STRM-02
**Success Criteria** (what must be TRUE):
  1. When an agent starts responding, tokens appear in the chat feed immediately and incrementally — the user sees a blinking cursor and text building character by character
  2. Streamed markdown renders progressively — tables and code blocks form correctly as tokens arrive, without layout jumps or broken partial renders
  3. Streaming does not drop tokens or corrupt messages — the final assembled message matches what a non-streaming response would have produced
**Plans**: 2 plans
**UI hint**: yes

Plans:
- [x] 09-01-PLAN.md — Streaming OpenAI client, callback delivery, platform delta endpoint
- [x] 09-02-PLAN.md — Frontend streaming store, cursor component, ChatMessage integration, end-to-end verification

### Phase 10: Inbound Webhooks
**Goal**: External systems can trigger TaskHub task creation via authenticated webhook endpoints, with GitHub and Slack as built-in integrations
**Depends on**: Phase 6 (independent of Phases 7-9)
**Requirements**: HOOK-01, HOOK-02, HOOK-03, HOOK-04
**Success Criteria** (what must be TRUE):
  1. A GitHub push or PR event with a valid HMAC signature creates a TaskHub task automatically — the task appears on the dashboard with the webhook payload as context
  2. A Slack slash command or event subscription triggers task creation with the Slack message content as the task description
  3. The webhook management page lets users create, edit, delete, and view webhook configurations with generated endpoint URLs and secrets
  4. Sending the same webhook delivery twice (provider retry) does not create duplicate tasks — idempotency protection works
**Plans**: TBD
**UI hint**: yes

## Progress

**Execution Order:**
Phases execute in numeric order: 7 → 8 → 9 → 10
(Phase 10 is independent and could run parallel to 8/9 if desired)

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation | v1.0 | 4/4 | Complete | 2026-04-05 |
| 2. Agent Status | v1.0 | 2/2 | Complete | 2026-04-05 |
| 3. Templates | v1.0 | 3/3 | Complete | 2026-04-06 |
| 4. Parallel Dashboard | v1.0 | 3/3 | Complete | 2026-04-06 |
| 5. Chat Intervention | v1.0 | 2/2 | Complete | 2026-04-06 |
| 6. Demo Readiness | v1.0 | 4/4 | Complete | 2026-04-07 |
| 7. Agent Tool Use | v2.0 | 0/2 | Planned | - |
| 8. Artifact Rendering | v2.0 | 0/2 | Planned | - |
| 9. Streaming Output | v2.0 | 0/2 | Planned | - |
| 10. Inbound Webhooks | v2.0 | 0/TBD | Not started | - |
