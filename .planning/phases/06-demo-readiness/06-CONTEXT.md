# Phase 6: Demo Readiness - Context

**Gathered:** 2026-04-06
**Status:** Ready for planning

<domain>
## Phase Boundary

All manage pages are functional with real or seeded data, the platform can run with OpenAI key for task decomposition, and every page is demo-worthy. Includes LLM provider switch, seed data for templates/policies, analytics per-agent drill-down with filters, audit time filtering, and a demo task seeding script.

</domain>

<decisions>
## Implementation Decisions

### LLM Provider Strategy
- **D-01:** Replace `claude` CLI exec in `orchestrator.go` with OpenAI Go SDK — single provider, no Anthropic SDK. `OPENAI_API_KEY` env var required for task execution.
- **D-02:** Use `gpt-4o-mini` as the default model for both task decomposition (`Plan`) and intent detection (`DetectIntent`)
- **D-03:** Remove all `claude` CLI references from orchestrator — clean break, not a dual-provider setup

### Seed Data — Templates
- **D-04:** Seed 5+ workflow templates with rich data: steps, variables, usage stats (usage_count, success_rate, avg_duration), multiple versions, mix of active/inactive states
- **D-05:** Templates should cover realistic business scenarios (e.g., Code Review, Market Research, Bug Triage, Content Creation, Onboarding Checklist, Security Audit)
- **D-06:** Each template has at least 2-3 steps with agent assignments and variable placeholders

### Seed Data — Policies
- **D-07:** Seed 4-5 policies with different priorities, rules JSON structures, and active/inactive states
- **D-08:** Policies should represent realistic governance scenarios (e.g., Security Review Required, Budget Approval Threshold, Data Access Policy, Compliance Check, Rate Limiting)

### Seed Data — Demo Task Script
- **D-09:** Provide a `make seed-demo` command that creates and executes several tasks through the platform, generating real analytics and audit data
- **D-10:** The script should produce enough data to make analytics dashboard and audit log pages look populated and functional

### Analytics Per-Agent Drill-Down
- **D-11:** Click an agent row in the performance table to expand an inline detail panel showing: task list for that agent, success/failure breakdown, average duration per task
- **D-12:** Add time range filter with preset ranges (7d / 30d / all) and status filter dropdown (completed / failed / all) to the analytics dashboard
- **D-13:** Filters affect both the stat cards and the agent performance table

### Audit Time Filtering
- **D-14:** Add preset time range buttons to the audit log: "Last hour" / "Today" / "7 days" / "30 days" / "All"
- **D-15:** Time filter works alongside existing event type / task ID / agent ID filters

### Claude's Discretion
- Exact template step content and variable definitions
- Policy rules JSON structure and field names
- Demo task script implementation approach (Go binary, shell script, or Makefile target calling API)
- Inline agent detail panel layout and styling
- Exact preset time range label text and button styling
- OpenAI SDK error handling and retry strategy

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Orchestrator (LLM replacement target)
- `internal/orchestrator/orchestrator.go` — callLLM function (line 134-147) uses `claude` CLI exec, Plan and Replan methods, DetectIntent method
- `cmd/openaiagent/openai.go` — Reference for OpenAI Go SDK usage pattern already in codebase

### Seed Infrastructure
- `internal/seed/devseed.go` — Current seed logic (only creates local user), extend with templates/policies/demo data
- `internal/handlers/templates.go` — Template CRUD handler (backend exists)
- `internal/handlers/policies.go` — Policy CRUD handler (backend exists)

### Manage Pages (Frontend)
- `web/app/manage/templates/page.tsx` — Template list page, calls `api.templates.list()`
- `web/app/manage/templates/[id]/page.tsx` — Template detail page
- `web/app/manage/analytics/page.tsx` — Re-exports from `web/app/analytics/page.tsx`
- `web/app/analytics/page.tsx` — Full analytics dashboard with stat cards, status distribution, daily tasks, agent performance table
- `web/app/manage/audit/page.tsx` — Re-exports from `web/app/audit/page.tsx`
- `web/app/audit/page.tsx` — Audit log with type/task/agent filters, pagination, expandable rows
- `web/app/manage/settings/policies/page.tsx` — Re-exports from `web/app/settings/policies/page.tsx`
- `web/app/settings/policies/page.tsx` — Policy CRUD page

### API Client
- `web/lib/api.ts` — API client with templates, policies, analytics, auditLogs namespaces (lines 105-140)
- `web/lib/types.ts` — TypeScript interfaces for WorkflowTemplate, Policy, DashboardData, AuditLogEntry, PaginatedAuditLogs

### Backend Handlers
- `internal/handlers/analytics.go` — Analytics dashboard endpoint
- `internal/handlers/auditlog.go` — Audit log list endpoint with filtering

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/openaiagent/openai.go` — OpenAI Go SDK integration already exists for team agents; pattern can be reused for orchestrator
- `internal/seed/devseed.go` — Existing seed infrastructure with idempotent upsert pattern
- `web/app/analytics/page.tsx` — Complete analytics dashboard with stat cards, bar chart, agent performance table — extend with drill-down and filters
- `web/app/audit/page.tsx` — Complete audit log with expandable rows, pagination — extend with time filter buttons
- `web/app/settings/policies/page.tsx` — Full CRUD policy page with create form, toggle, delete

### Established Patterns
- Go handlers: thin handlers delegating to service packages
- Frontend: shadcn/ui + Tailwind CSS + Zustand stores
- API client: typed functions returning Promise<T>
- Seed data: idempotent upsert with `ON CONFLICT DO NOTHING`

### Integration Points
- `orchestrator.go:callLLM` — Primary replacement target for OpenAI SDK
- `devseed.go` — Extend with template/policy seeding functions
- `analytics.go` — Backend needs new endpoints for filtered data and per-agent drill-down
- `auditlog.go` — Backend needs time range query parameter support
- `Makefile` — Add `seed-demo` target

</code_context>

<specifics>
## Specific Ideas

- Demo should look like a real product with populated data on every manage page
- The `make seed-demo` approach generates authentic data through actual task execution rather than fake database inserts
- gpt-4o-mini chosen for cost efficiency in demo scenarios

</specifics>

<deferred>
## Deferred Ideas

- Dual LLM provider support (Anthropic + OpenAI switchable) — could be future enhancement but not needed for demo
- Custom date picker for audit log — preset ranges are sufficient for demo
- Agent detail analytics page (separate route) — inline expand is enough for demo

None — discussion stayed within phase scope

</deferred>

---

*Phase: 06-demo-readiness*
*Context gathered: 2026-04-06*
