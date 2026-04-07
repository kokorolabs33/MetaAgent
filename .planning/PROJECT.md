# TaskHub

## What This Is

TaskHub is an open-source A2A (Agent-to-Agent) protocol meta-agent platform that demonstrates how companies can use the A2A protocol for collaborative multi-agent task completion. It provides a Master Agent that decomposes user requests into subtask DAGs, orchestrates specialized sub-agents via A2A protocol, and offers real-time observability of the entire collaboration process. Built with Go + Next.js + PostgreSQL, targeting the developer community as a reference implementation and exploration of A2A-powered workflows.

## Core Value

Developers can experience a complete A2A multi-agent collaboration flow — from chat-driven task creation through real-time observation of agent coordination to task completion — in a polished, open-source package they can clone and run.

## Requirements

### Validated

- ✓ Task creation and LLM-driven decomposition into subtask DAGs — existing
- ✓ A2A protocol agent communication (HTTP polling + native adapters) — existing
- ✓ SSE real-time event streaming to frontend — existing
- ✓ DAG visualization with React Flow — existing
- ✓ Agent registration and management — existing
- ✓ @mention cross-agent collaboration — existing
- ✓ Audit logging with token counts and cost estimates — existing
- ✓ Policy-driven execution with approval workflows — existing
- ✓ Adaptive replanning on subtask failure — existing
- ✓ Conversation memory per contextId for multi-turn interactions — existing
- ✓ Chat interface with Master Agent — existing
- ✓ Agent status visualization (online/working/idle/offline) — Validated in Phase 2
- ✓ Enhanced chat intervention (advisory @mention to sub-agents) — Validated in Phase 5
- ✓ Multi-task parallel dashboard with live status — Validated in Phase 4
- ✓ OpenAI LLM support (gpt-4o-mini) — Validated in Phase 6
- ✓ Seeded demo data (templates, policies, demo tasks) — Validated in Phase 6
- ✓ Analytics per-agent drill-down with time/status filters — Validated in Phase 6
- ✓ Audit log time range filtering — Validated in Phase 6

### Active

- [ ] Agent tool use — OpenAI function calling with web search as first tool, agents do real work not just chat
- [ ] Artifact rich rendering — agents produce structured outputs (tables, reports, diffs) as rich UI cards
- [ ] Streaming agent output — real-time token streaming so users watch agents think and write
- [ ] Inbound webhooks — trigger tasks from external events (Slack, GitHub, etc.)

### Out of Scope

- External A2A agent registration flow overhaul — current registration workflow is sufficient
- Commercial SaaS features (billing, multi-tenancy, auth providers) — this is an open-source showcase
- Mobile app or responsive mobile UI — desktop-first developer tool
- Real-time video/voice communication between agents — text-based A2A protocol focus

## Current Milestone: v2.0 Wow Moment

**Goal:** Make agents do real work and produce visible results — transform the demo from "agents chatting" to "agents working with real data and producing actionable outputs"

**Target features:**
- Agent Tool Use — OpenAI function calling with web search as first tool
- Artifact Rich Rendering — agents produce structured outputs (tables, reports, code diffs) rendered as rich cards
- Streaming Agent Output — real-time token streaming so users watch agents think and write
- Inbound Webhooks — trigger tasks from external events (Slack, GitHub, etc.)

## Context

- v1.0 shipped: foundation, agent status, parallel dashboard, chat intervention, demo readiness (6 phases complete)
- A2A protocol ecosystem is still nascent with few reference implementations; TaskHub aims to fill this gap
- Current agents are chat-only — no tool use, no structured outputs, no external data access
- The demo needs a "wow moment" to differentiate from single-LLM chat experiences
- Target audience is developers and technical community (GitHub open-source)
- One-person-company philosophy: lean, focused, high-quality rather than feature-bloated

## Constraints

- **Tech stack**: Go 1.26 + Next.js 16 + PostgreSQL + pgx — maintain existing stack, new dependencies allowed if justified
- **A2A compliance**: Must remain compatible with A2A protocol specification (JSON-RPC 2.0)
- **Database**: PostgreSQL with embedded migrations — no ORM, raw SQL via pgx
- **Frontend patterns**: shadcn/ui + Tailwind CSS + Zustand stores — maintain consistency
- **Quality gates**: `make check` must pass (format + lint + typecheck + build) before any merge

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Chat-driven interaction model (like golutra) | Developers expect conversational UX; aligns with A2A protocol's message-passing nature | — Pending |
| Parallel bug-fix + feature development | Both polish and new features needed for open-source readiness | — Pending |
| Borrow golutra patterns (status viz, dispatch) | Proven UX patterns from sister project, saves design iteration | — Pending |
| Keep existing agent registration flow | Current flow is functional, not worth reworking for v1 open-source release | ✓ Good |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-07 after milestone v2.0 start*
