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

- ✓ Agent tool use — OpenAI function calling with Tavily web search, per-role tool sets — Validated in Phase 7
- ✓ Artifact rich rendering — typed artifact cards (search, code, table, data) with copy/download, react-markdown GFM upgrade — Validated in Phase 8
- ✓ Streaming agent output — token-by-token streaming with progressive markdown and blinking cursor — Validated in Phase 9
- ✓ Inbound webhooks — HMAC-SHA256 authenticated endpoints with GitHub/Slack parsers and idempotency — Validated in Phase 10

### Active

(No active requirements — next milestone not yet planned)

### Out of Scope

- External A2A agent registration flow overhaul — current registration workflow is sufficient
- Commercial SaaS features (billing, multi-tenancy, auth providers) — this is an open-source showcase
- Mobile app or responsive mobile UI — desktop-first developer tool
- Real-time video/voice communication between agents — text-based A2A protocol focus

## Shipped: v2.0 Wow Moment (2026-04-07)

**Delivered:** Agents do real work — tool calling with web search, rich artifact rendering, token-by-token streaming, and inbound webhooks from GitHub/Slack. Transformed the demo from "agents chatting" to "agents working with real data and producing actionable outputs."

## Next Milestone

Not yet planned. Run `/gsd-new-milestone` to start.

## Context

- v1.0 shipped: foundation, agent status, parallel dashboard, chat intervention, demo readiness (6 phases)
- v2.0 shipped: agent tool use, artifact rendering, streaming output, inbound webhooks (4 phases, 8 plans)
- 10 phases complete across 2 milestones, ~8200 lines added
- Agents now call real tools (Tavily web search), produce structured outputs, stream token-by-token
- External systems (GitHub, Slack) can trigger tasks via HMAC-authenticated webhooks
- A2A protocol ecosystem is still nascent; TaskHub is a comprehensive reference implementation
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
| Chat-driven interaction model (like golutra) | Developers expect conversational UX; aligns with A2A protocol's message-passing nature | ✓ Good |
| Parallel bug-fix + feature development | Both polish and new features needed for open-source readiness | ✓ Good |
| Borrow golutra patterns (status viz, dispatch) | Proven UX patterns from sister project, saves design iteration | ✓ Good |
| Keep existing agent registration flow | Current flow is functional, not worth reworking for v1 open-source release | ✓ Good |
| Tavily API for web search (hand-rolled HTTP) | Free tier, simple REST API, no SDK dependency | ✓ Good — v2.0 |
| Hardcoded tool registry (no DB) | Tools are code-level, not user-configured — simpler, faster | ✓ Good — v2.0 |
| react-markdown + remark-gfm for all messages | Replaces hand-rolled markdown, consistent GFM rendering | ✓ Good — v2.0 |
| Single-layer streaming (agent → platform → browser) | Simpler than full A2A streaming; sufficient for demo | ✓ Good — v2.0 |
| HMAC-SHA256 with dual-secret rotation | Industry standard webhook auth with zero-downtime key rotation | ✓ Good — v2.0 |

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
*Last updated: 2026-04-07 after milestone v2.0 completion*
