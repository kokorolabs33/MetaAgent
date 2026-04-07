# Phase 6: Demo Readiness - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-06
**Phase:** 06-demo-readiness
**Areas discussed:** LLM Provider Strategy, Seed Data Content, Analytics Drill-Down, Audit Time Filtering

---

## LLM Provider Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Only OpenAI | Replace claude CLI with OpenAI Go SDK. Single provider, simplest. OPENAI_API_KEY required. | |
| Dual provider switching | Anthropic SDK + OpenAI SDK, LLM_PROVIDER env var. More flexible but complex. | |
| OpenAI-compatible API | OpenAI SDK as sole client, base URL configurable for any compatible service. | |

**User's choice:** Only OpenAI
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| gpt-4o | Best value, sufficient for decomposition. Cheap. | |
| gpt-4o-mini | Cheaper and faster, slightly less stable JSON output. Good for demo. | |
| Configurable | Default gpt-4o, OPENAI_MODEL env var override. | |

**User's choice:** gpt-4o-mini
**Notes:** None

---

## Seed Data Content

### Templates

| Option | Description | Selected |
|--------|-------------|----------|
| 3-4 business templates | Code Review, Market Research, Bug Triage, Content Creation. Each with steps, variables, usage stats. | |
| 1-2 minimal | Just enough for non-empty list. | |
| 5+ rich data | Multiple templates with rich usage stats, multiple versions, active/inactive states. | |

**User's choice:** 5+ rich data
**Notes:** None

### Policies

| Option | Description | Selected |
|--------|-------------|----------|
| 2-3 example policies | Security Review Required, Budget Approval, Data Access Policy. Active/inactive. | |
| No seed needed | Policies page has Create button, empty list + create is demable. | |
| 4-5 rich data | Multiple policies with different priority, rules JSON, active/inactive. | |

**User's choice:** 4-5 rich data
**Notes:** None

### Additional Seed Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Only templates + policies | Analytics/audit depend on real task data, no extra seed needed. | |
| Add historical task data | Seed completed tasks + subtasks for analytics/audit population. More work. | |
| Templates + policies + demo task script | make seed-demo command, auto-creates tasks for agents to execute, generates real data. | |

**User's choice:** Templates + policies + demo task script
**Notes:** None

---

## Analytics Drill-Down

| Option | Description | Selected |
|--------|-------------|----------|
| Click agent row to expand inline | Inline panel with task list, success/failure stats, average duration. Simple, stays on page. | |
| Separate agent detail page | Navigate to /manage/analytics/agent/[id]. Richer but more work. | |
| Keep current table | Existing agent performance table is sufficient for demo. | |

**User's choice:** Click agent row to expand inline
**Notes:** None

### Analytics Filters

| Option | Description | Selected |
|--------|-------------|----------|
| Time range + status filter | Preset ranges (7d/30d/all) + status dropdown (completed/failed/all). Required by success criteria. | |
| Time range only | Preset ranges sufficient, no status filter. | |
| No filters | Current page is sufficient. | |

**User's choice:** Time range + status filter
**Notes:** None

---

## Audit Time Filtering

| Option | Description | Selected |
|--------|-------------|----------|
| Preset range buttons | "Last hour" / "Today" / "7 days" / "30 days" / "All" buttons. Simple, demo-friendly. | |
| Date picker | Start/end date selector. More flexible but needs extra UI component. | |
| Preset + date picker | Both options. Most flexible, most work. | |

**User's choice:** Preset range buttons
**Notes:** None

---

## Claude's Discretion

- Exact template step content and variable definitions
- Policy rules JSON structure
- Demo task script implementation approach
- Inline agent detail panel layout
- Preset time range button styling
- OpenAI SDK error handling

## Deferred Ideas

- Dual LLM provider support (Anthropic + OpenAI) — future enhancement
- Custom date picker for audit — preset ranges sufficient
- Separate agent analytics page — inline expand is enough
