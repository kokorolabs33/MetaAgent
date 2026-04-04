# Feature Landscape

**Domain:** Open-source A2A multi-agent orchestration platform (developer showcase)
**Researched:** 2026-04-04
**Confidence:** HIGH (existing codebase verified + ecosystem cross-referenced)

---

## Context: What TaskHub Already Has

The following are already implemented and NOT categorized below (they are baseline):

- Task creation with LLM-driven DAG decomposition
- A2A protocol agent communication (HTTP polling + native adapters)
- SSE real-time event streaming
- DAG visualization (React Flow)
- Agent registration and management
- @mention cross-agent collaboration (up to 3 rounds)
- Audit logging with token counts and cost estimates
- Policy-driven execution with approval workflows
- Adaptive replanning on subtask failure
- Conversation memory per contextId
- Master Agent chat interface

This document focuses exclusively on what comes **next**.

---

## Table Stakes

Features a developer cloning TaskHub will expect. Missing any of these and the repo reads as unfinished or hard to evaluate.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Agent status indicators** (online/idle/working/offline) | Every agent platform — LangSmith, CrewAI, MindStudio — shows live agent state. Developers expect to see if agents are reachable before trusting the system | Low–Med | Color-coded dot + label; pulse animation when working. Driven by existing health-checker output. Needs SSE event bridge to frontend |
| **One-click local startup** (Docker Compose or `make dev`) | Top open-source projects (n8n, Langfuse, Open Agent Platform) all gate GitHub stars on "clone and it works in 2 minutes". TaskHub's current multi-step setup is a barrier | Low | `docker compose up` with seeded demo agents. README hero section. This is the #1 open-source adoption factor |
| **Comprehensive README with live demo GIF/screenshot** | GitHub's top agentic repos (AutoGen 32k stars, LangGraph 24.8k stars) all feature hero demos. Without one, visitors bounce in 10 seconds | Low | GIF/screenshot of DAG execution + agent chat in action. Badges: build status, license, Go version |
| **Frontend bug-free interaction** | Broken UI on a demo repo signals "not maintained". Developers judge code quality by what they can click | Med | Existing known bugs must be fixed before any feature work ships to main. This blocks demo-readiness |
| **Basic task filtering/search on dashboard** | Standard in every workflow tool (Prefect, Airflow). Developers running multiple tasks need to find them | Low | Filter by status (pending/running/completed/failed). Client-side is fine for v1 |
| **Task detail — subtask timeline/trace view** | LangSmith and Langfuse both surface execution traces as the primary debug surface. Developers will look for "what did each agent actually do?" | Med | Chronological timeline of subtask start/end with agent name, duration, truncated output. `TraceTimeline.tsx` already exists in working tree |
| **Cost/token display per task** | LLM cost awareness is now expected developer hygiene (confirmed by LangSmith + Langfuse both making it prominent). Audit log endpoint already exists | Low | Show total tokens + estimated cost on task detail. Data is already captured in audit log |

**Dependency chain:** Bug-free interaction → one-click startup → README/demo. These three must ship together as a "GitHub readiness" batch before any feature work matters.

---

## Differentiators

Features that make TaskHub stand apart from the crowded agent framework space. None of these are expected by default — they are the reason a developer stars, forks, or writes about the repo.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **User-to-sub-agent direct chat during execution** | No existing open-source platform supports real-time user intervention on a running sub-agent. LangGraph, CrewAI, AutoGen all treat sub-agents as black boxes during execution. A2A's message-passing model makes this uniquely natural for TaskHub | High | User sends message to @agent_id in conversation during active task. Handler routes to A2A aggregator mid-execution. Requires conversation → subtask context link and A2A mid-task message injection. This is TaskHub's flagship differentiator |
| **Multi-task parallel view dashboard** | VS Code's new multi-agent dev view (2026) showed that "see all running agents at once" cuts debugging time 25% in beta. No open-source A2A platform has a unified parallel execution view | Med | Grid/list of active tasks showing: status badge, running subtask count, active agent names, last event. Click-through to full DAG. Driven by existing SSE infrastructure |
| **Task templates with execution accumulation** | Saving successful orchestration patterns is a gap in all current open-source platforms. Airflow/Prefect have DAG reuse but no LLM-driven pattern learning. TaskHub's template + replan history creates a learning loop no competitor has | Med–High | Two parts: (1) "Save this orchestration as template" button — stores agent assignments + DAG shape. (2) Template auto-match on new tasks — suggest template when title similarity is high. `WorkflowTemplate` model already in DB. Part 1 is Med; part 2 is High |
| **A2A protocol showcase clarity** | The A2A protocol ecosystem has few real multi-agent reference implementations. Google's own samples demonstrate single-agent interactions. A clear "this is how A2A works end-to-end" narrative in the UI (show agent card, show JSON-RPC wire calls in a dev panel) makes TaskHub the canonical learning resource | Med | Dev panel toggle (hidden by default) showing: agent-card.json, raw A2A JSON-RPC call/response for selected subtask. Requires no new backend — data already in events/audit. Purely a frontend addition |
| **Adaptive replanning visibility** | TaskHub already does replanning — but it's invisible. Surfacing "Subtask S3 failed, replanned with new approach" in the UI is unique. No public platform shows LLM-driven failure recovery in real-time | Low–Med | Event type `task.replanned` surfaced as a special timeline entry with old vs. new subtask diff. Mostly frontend work on top of existing replan events |

---

## Anti-Features

Things to explicitly NOT build for this milestone. Each has a clear reason.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **OpenTelemetry / external observability integration** | LangFuse and LangSmith already do this well. Building OTEL export is weeks of work for zero differentiation. TaskHub's audit log is sufficient for a showcase | Keep audit log as-is. Link to LangFuse in README as "production observability" recommendation |
| **Visual workflow builder (drag-and-drop DAG editor)** | n8n has 150k+ stars on this. Competing with a visual builder is a years-long effort. TaskHub's value is LLM-driven DAG creation, not manual workflow construction | Emphasize that the LLM does the decomposition — that IS the point |
| **Multi-tenancy / org management** | SaaS-tier feature. Out of scope per PROJECT.md. Adds auth complexity that obscures the A2A showcase | Single-user local mode is the right default. Google OAuth for cloud deployment is sufficient |
| **Plugin/extension system for custom agent types** | Premature generalization. Extension points before the core is polished lead to abandonment. CrewAI's complexity is cited as a barrier; TaskHub should be the opposite | Use A2A protocol as the extension mechanism — any HTTP agent is a plugin |
| **Evaluation / LLM-as-judge scoring** | Langfuse has full eval infrastructure. Building evals from scratch is weeks of work with no demo payoff | Display raw task output clearly; let developers judge quality themselves |
| **Mobile-responsive UI** | Desktop-first developer tool per PROJECT.md. Responsive polish at this stage is wasted effort | Focus desktop layout; don't break mobile but don't optimize for it |
| **Agent marketplace / registry** | Premature. Community adoption must come first. A2A agent card + discovery endpoint is the right foundation; a marketplace is a product layer that needs an audience | Surface the existing agent-card.json endpoint clearly in the README as the discovery mechanism |
| **Real-time collaboration (multiple users watching same task)** | Complex WebSocket infrastructure (presence, conflict resolution) for a feature that barely matters in a single-developer showcase tool | SSE per-user is sufficient; don't add multi-cursor complexity |

---

## Feature Dependencies

```
Bug-free interaction
  → one-click startup (can't demo broken UI)
    → README/demo GIF (needs working product to record)
      → GitHub open-source launch

Agent status indicators
  → health-checker SSE bridge (backend)
    → status dot component (frontend)

User-to-sub-agent direct chat
  → conversation-to-subtask context link
    → A2A mid-task message injection (backend)
      → chat UI routing to @agent mentions during active task

Task templates
  → "save as template" (part 1)
    → template auto-match on new task (part 2, optional v1)

Multi-task parallel view
  → no new backend dependencies
  → requires stable SSE (fixed in bug-free interaction pass)

A2A showcase dev panel
  → no new backend
  → requires existing event/audit data to be well-structured
```

---

## MVP Recommendation

For the active milestone (github open-source readiness + featured additions), prioritize in this order:

**Must ship (table stakes):**
1. Frontend bug fixes — unblocks everything
2. One-click Docker startup + seeded demo agents
3. Agent status indicators (visible proof the system is live)
4. README with hero demo GIF + quickstart instructions
5. Task trace/timeline view (developers will look for this immediately)
6. Cost/token display (existing data, minimal work)

**High-value differentiators (ship if time allows):**
7. Multi-task parallel view dashboard — medium complexity, high visual impact for demos
8. Adaptive replanning visibility — mostly frontend, unique to TaskHub
9. A2A dev panel (show JSON-RPC wire calls) — makes TaskHub the canonical A2A learning resource
10. User-to-sub-agent direct chat — highest complexity, flagship differentiator; target for follow-on milestone if scope is tight

**Defer:**
- Task templates (part 2: auto-match) — defer until templates are being used
- Task filtering/search — can ship alongside parallel view as simple filter buttons

---

## Sources

- [A2A Protocol specification](https://a2a-protocol.org/latest/)
- [A2A Protocol GitHub samples](https://github.com/a2aproject/a2a-samples)
- [Google A2A Announcement](https://developers.googleblog.com/en/a2a-a-new-era-of-agent-interoperability/)
- [LangGraph vs CrewAI vs AutoGen comparison 2026](https://o-mega.ai/articles/langgraph-vs-crewai-vs-autogen-top-10-agent-frameworks-2026)
- [Langfuse open source observability](https://langfuse.com/docs/observability/overview)
- [AgentOps agent observability features](https://aiagentslist.com/agents/agentops)
- [Agentic UX patterns — Smashing Magazine](https://www.smashingmagazine.com/2026/02/designing-agentic-ai-practical-ux-patterns/)
- [VS Code multi-agent development (2026)](https://code.visualstudio.com/blogs/2026/02/05/multi-agent-development)
- [Building effective AI agents — Anthropic](https://www.anthropic.com/research/building-effective-agents)
- [Simon Willison agentic anti-patterns](https://simonwillison.net/guides/agentic-engineering-patterns/anti-patterns/)
- [Top open-source agentic AI repos 2025](https://opendatascience.com/the-top-ten-github-agentic-ai-repositories-in-2025/)
- [n8n AI agent orchestration frameworks comparison](https://blog.n8n.io/ai-agent-orchestration-frameworks/)
