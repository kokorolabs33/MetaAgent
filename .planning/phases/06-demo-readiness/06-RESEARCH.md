# Phase 6: Demo Readiness - Research

**Researched:** 2026-04-06
**Domain:** LLM provider replacement (OpenAI), seed data strategy, analytics/audit backend enhancements
**Confidence:** HIGH

## Summary

Phase 6 transforms TaskHub from a development-stage platform into a demo-ready product by addressing three domains: (1) replacing the `claude` CLI exec in the orchestrator with direct OpenAI API calls, (2) seeding templates, policies, and demo task data so every manage page is populated, and (3) enhancing the analytics and audit backends with filtering and drill-down capabilities.

The codebase already contains a working OpenAI HTTP client pattern in `cmd/openaiagent/openai.go` that can be extracted into a shared package. The database schema already supports templates, policies, template_executions, and events -- no new migrations are required. The analytics and audit endpoints need query parameter extensions (time range, status filters, agent drill-down) but the core query patterns are established.

**Primary recommendation:** Extract the existing OpenAI HTTP client into a shared `internal/llm` package, extend analytics/audit handlers with filter parameters, create seed functions in `internal/seed/`, and add a `make seed-demo` target.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Replace `claude` CLI exec in `orchestrator.go` with OpenAI Go SDK -- single provider, no Anthropic SDK. `OPENAI_API_KEY` env var required for task execution.
- **D-02:** Use `gpt-4o-mini` as the default model for both task decomposition (`Plan`) and intent detection (`DetectIntent`)
- **D-03:** Remove all `claude` CLI references from orchestrator -- clean break, not a dual-provider setup
- **D-04:** Seed 5+ workflow templates with rich data: steps, variables, usage stats (usage_count, success_rate, avg_duration), multiple versions, mix of active/inactive states
- **D-05:** Templates should cover realistic business scenarios (e.g., Code Review, Market Research, Bug Triage, Content Creation, Onboarding Checklist, Security Audit)
- **D-06:** Each template has at least 2-3 steps with agent assignments and variable placeholders
- **D-07:** Seed 4-5 policies with different priorities, rules JSON structures, and active/inactive states
- **D-08:** Policies should represent realistic governance scenarios (e.g., Security Review Required, Budget Approval Threshold, Data Access Policy, Compliance Check, Rate Limiting)
- **D-09:** Provide a `make seed-demo` command that creates and executes several tasks through the platform, generating real analytics and audit data
- **D-10:** The script should produce enough data to make analytics dashboard and audit log pages look populated and functional
- **D-11:** Click an agent row in the performance table to expand an inline detail panel showing: task list for that agent, success/failure breakdown, average duration per task
- **D-12:** Add time range filter with preset ranges (7d / 30d / all) and status filter dropdown (completed / failed / all) to the analytics dashboard
- **D-13:** Filters affect both the stat cards and the agent performance table
- **D-14:** Add preset time range buttons to the audit log: "Last hour" / "Today" / "7 days" / "30 days" / "All"
- **D-15:** Time filter works alongside existing event type / task ID / agent ID filters

### Claude's Discretion
- Exact template step content and variable definitions
- Policy rules JSON structure and field names
- Demo task script implementation approach (Go binary, shell script, or Makefile target calling API)
- Inline agent detail panel layout and styling
- Exact preset time range label text and button styling
- OpenAI SDK error handling and retry strategy

### Deferred Ideas (OUT OF SCOPE)
- Dual LLM provider support (Anthropic + OpenAI switchable)
- Custom date picker for audit log
- Agent detail analytics page (separate route)
</user_constraints>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/http` | stdlib | OpenAI API calls | Already used in `cmd/openaiagent/openai.go`; no new dependency needed [VERIFIED: go.mod] |
| `pgx/v5` | v5.9.1 | Database queries for seed data, analytics, audit | Already in go.mod [VERIFIED: go.mod] |
| `google/uuid` | v1.6.0 | UUID generation for seeded records | Already in go.mod [VERIFIED: go.mod] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Raw HTTP for OpenAI | `github.com/openai/openai-go/v3` (official SDK v3.30.0) | Official SDK adds structured output helpers and auto-retry, but adds a new dependency to a 5-dependency project; the codebase already has a working raw HTTP pattern that matches the project's minimal-dependency philosophy [VERIFIED: github.com/openai/openai-go] |
| Raw HTTP for OpenAI | `github.com/sashabaranov/go-openai` (community SDK) | Community SDK is popular but unofficial; same dependency trade-off as official SDK [VERIFIED: pkg.go.dev] |

**Recommendation:** Reuse the existing raw HTTP client pattern from `cmd/openaiagent/openai.go`. The orchestrator only needs Chat Completions with a system prompt and user message -- exactly what `OpenAIClient.Chat()` already does. Extract it into `internal/llm/openai.go` so both the orchestrator and the openaiagent command can use it. This adds zero new dependencies. [ASSUMED -- architecture choice within Claude's discretion]

**No new packages need to be installed.** All required functionality exists in the current dependency tree.

## Architecture Patterns

### Recommended Project Structure Changes
```
internal/
├── llm/                     # NEW: Shared LLM client
│   └── openai.go            # Extracted from cmd/openaiagent/openai.go
├── orchestrator/
│   └── orchestrator.go      # Modified: callLLM uses internal/llm instead of claude CLI
├── seed/
│   ├── devseed.go           # Existing: local user seed
│   ├── templates.go         # NEW: Template seed data
│   ├── policies.go          # NEW: Policy seed data
│   └── demo.go              # NEW: Demo task execution seed
└── handlers/
    ├── analytics.go          # Modified: add filter params, agent drill-down endpoint
    └── auditlog.go           # Modified: add time range filter
```

### Pattern 1: OpenAI Client Extraction
**What:** Move `OpenAIClient` from `cmd/openaiagent/openai.go` into `internal/llm/openai.go` so the orchestrator can use it.
**When to use:** When the orchestrator's `callLLM` function is replaced.
**Key change:** The orchestrator currently uses `exec.CommandContext(ctx, "claude", ...)` -- replace with `llmClient.Chat(ctx, systemPrompt, userMsg)`.

```go
// internal/llm/openai.go
// Source: Extracted from cmd/openaiagent/openai.go (verified in codebase)
package llm

type Client struct {
    apiKey     string
    model      string
    httpClient *http.Client
}

func NewClient() *Client {
    apiKey := os.Getenv("OPENAI_API_KEY")
    model := os.Getenv("OPENAI_MODEL")
    if model == "" {
        model = "gpt-4o-mini" // D-02 default
    }
    return &Client{apiKey: apiKey, model: model, httpClient: &http.Client{}}
}

func (c *Client) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
    // Same implementation as cmd/openaiagent/openai.go Chat method
}
```

**Orchestrator integration:**
```go
// internal/orchestrator/orchestrator.go
type Orchestrator struct {
    LLM *llm.Client // Injected dependency
}

func callLLM(ctx context.Context, client *llm.Client, systemPrompt, userMsg string) (string, error) {
    response, err := client.Chat(ctx, systemPrompt, userMsg)
    if err != nil {
        return "", fmt.Errorf("openai: %w", err)
    }
    return stripMarkdownFences(strings.TrimSpace(response)), nil
}
```

### Pattern 2: Idempotent Seed Data
**What:** Use `INSERT ... ON CONFLICT DO NOTHING` (or `ON CONFLICT DO UPDATE`) for all seed data.
**When to use:** All seed functions in `internal/seed/`.
**Why:** Server restarts should not duplicate data. Matches existing pattern in `devseed.go`.

```go
// Source: Existing pattern from internal/seed/devseed.go (verified in codebase)
_, err := pool.Exec(ctx,
    `INSERT INTO workflow_templates (id, name, description, version, steps, variables, is_active, created_at, updated_at)
     VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
     ON CONFLICT (name) DO NOTHING`,
    id, name, desc, version, steps, variables, isActive, now, now)
```

The `workflow_templates` table has `UNIQUE (name)` constraint, so `ON CONFLICT (name) DO NOTHING` is safe. [VERIFIED: migration 005]

### Pattern 3: Analytics with Query Parameters
**What:** Extend `GetDashboard` to accept `?range=7d|30d|all` and `?status=completed|failed|all` query params that filter all aggregate queries.
**When to use:** Analytics dashboard endpoint.
**Implementation:** Add WHERE clauses to all existing queries based on the time range and status params.

```go
// Parameterized time range filter
func timeRangeCondition(rangeParam string) (string, []any) {
    switch rangeParam {
    case "7d":
        return "created_at > NOW() - INTERVAL '7 days'", nil
    case "30d":
        return "created_at > NOW() - INTERVAL '30 days'", nil
    default:
        return "", nil // "all" = no time restriction
    }
}
```

### Pattern 4: Agent Drill-Down Endpoint
**What:** New endpoint `GET /api/analytics/agents/{id}/tasks` returning subtask-level data for a specific agent.
**When to use:** When the frontend expands an agent row in the performance table.
**Query params:** `?range=7d|30d|all&status=completed|failed|all`

```go
// Returns subtasks assigned to agent with task title, status, duration
`SELECT s.id, t.title, s.status,
        EXTRACT(EPOCH FROM (s.completed_at - s.started_at)) AS duration_sec,
        s.created_at
 FROM subtasks s
 JOIN tasks t ON t.id = s.task_id
 WHERE s.agent_id = $1
 {timeFilter}
 {statusFilter}
 ORDER BY s.created_at DESC
 LIMIT 50`
```

### Pattern 5: Audit Time Range Filter
**What:** Add `?since=` query parameter to the existing audit log endpoint.
**When to use:** When the frontend sends a time range preset.
**Implementation:** Map preset labels to interval strings.

```go
// The frontend sends ?since=1h|today|7d|30d
// Backend maps to WHERE condition on events.created_at
switch since {
case "1h":
    conditions = append(conditions, fmt.Sprintf("created_at > NOW() - INTERVAL '1 hour'"))
case "today":
    conditions = append(conditions, fmt.Sprintf("created_at > DATE_TRUNC('day', NOW())"))
case "7d":
    conditions = append(conditions, fmt.Sprintf("created_at > NOW() - INTERVAL '7 days'"))
case "30d":
    conditions = append(conditions, fmt.Sprintf("created_at > NOW() - INTERVAL '30 days'"))
// default: no filter ("all")
}
```

This fits cleanly into the existing dynamic WHERE clause builder in `auditlog.go`. [VERIFIED: internal/handlers/auditlog.go lines 37-55]

### Anti-Patterns to Avoid
- **Do not add `response_format: {"type": "json_object"}` to OpenAI calls:** The orchestrator already instructs the model to respond with "ONLY valid JSON" in the prompt. Adding `response_format` would add complexity for marginal benefit on `gpt-4o-mini` which already follows JSON instructions well. The existing `stripMarkdownFences` fallback handles edge cases. [ASSUMED]
- **Do not seed fake analytics directly into DB:** D-09/D-10 specify that demo data should come from actual task execution through the platform API, not synthetic database inserts. This generates authentic events and audit trail data.
- **Do not create a separate analytics service:** All analytics queries are simple SQL aggregates -- keep them in the handler.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| OpenAI API client | New SDK integration | Extract existing `cmd/openaiagent/openai.go` | Already tested, zero new dependencies |
| UUID generation | Custom ID scheme | `github.com/google/uuid` | Already in project |
| JSON parsing | Custom JSON parsing | `encoding/json` stdlib | Standard Go |
| Time range SQL | Complex date math | PostgreSQL `NOW() - INTERVAL` | Database handles timezone correctly |
| Template usage stats | Denormalized counter columns | `COUNT/AVG` over `template_executions` join | Avoids sync issues between counter and actual data |

**Key insight:** This phase is primarily about connecting existing pieces -- the OpenAI client already exists, the DB schema already supports templates/policies/executions, the frontend pages already exist with empty states. The work is extraction, wiring, and data population.

## Common Pitfalls

### Pitfall 1: OpenAI API Key Missing at Startup
**What goes wrong:** Server starts but task execution fails because `OPENAI_API_KEY` is not set.
**Why it happens:** The existing config doesn't load or validate this key.
**How to avoid:** Add `OPENAI_API_KEY` to `config.go`; log a warning at startup if empty (don't fatal -- the server should still serve pages, just not execute tasks).
**Warning signs:** "openai error: Incorrect API key provided" in logs.

### Pitfall 2: Template Seed Data Violates Unique Constraint
**What goes wrong:** `INSERT` fails on restart because template name already exists.
**Why it happens:** `workflow_templates.name` has a `UNIQUE` constraint.
**How to avoid:** Always use `ON CONFLICT (name) DO NOTHING` for seed inserts. [VERIFIED: migration 005 line 59]
**Warning signs:** `duplicate key value violates unique constraint "workflow_templates_name_key"` in logs.

### Pitfall 3: Analytics Filters Not Applied to All Queries
**What goes wrong:** Stat cards show all-time data while the agent table shows filtered data (or vice versa).
**Why it happens:** The handler runs 7+ separate SQL queries; easy to miss adding the WHERE clause to one.
**How to avoid:** Build the time/status filter conditions once, then inject them into every query in `GetDashboard`. Extract a helper function.
**Warning signs:** Numbers don't add up -- e.g., total_tasks != sum of status distribution counts.

### Pitfall 4: Template Usage Stats Require Join with template_executions
**What goes wrong:** Template list page shows templates but no usage stats.
**Why it happens:** The `workflow_templates` table has no `usage_count` column. Stats must be computed from `template_executions`.
**How to avoid:** Either (a) add a new endpoint `GET /api/templates` that joins with `template_executions` for aggregate stats, or (b) add a computed stats object to the template list response via a subquery.
**Warning signs:** Template cards show "0 uses" even after demo tasks execute with templates.

### Pitfall 5: Demo Seed Script Depends on Running Agents
**What goes wrong:** `make seed-demo` tries to create and execute tasks, but agent endpoints are unreachable.
**Why it happens:** The demo script calls `POST /api/tasks` which triggers orchestration, which invokes agents via A2A HTTP calls.
**How to avoid:** Two approaches: (a) the demo script only seeds static data (templates, policies) and creates tasks in "completed" state with pre-built plans and events, or (b) document that `make agents` must be running. Approach (a) is more reliable for demo purposes.
**Warning signs:** Tasks get stuck in "planning" or "running" state because agents are unreachable.

### Pitfall 6: `callLLM` Return Value Contains Markdown Fences
**What goes wrong:** JSON parsing fails after switching from `claude` CLI to OpenAI API.
**Why it happens:** Different models have different tendencies to wrap JSON in markdown fences.
**How to avoid:** Keep the existing `stripMarkdownFences` function. `gpt-4o-mini` sometimes wraps JSON in \`\`\`json blocks. [ASSUMED]
**Warning signs:** `parse plan response: invalid character` errors.

## Code Examples

### OpenAI Chat Completion (existing pattern)
```go
// Source: cmd/openaiagent/openai.go (verified in codebase)
func (c *OpenAIClient) Chat(ctx context.Context, systemPrompt string, userMessage string) (string, error) {
    return c.ChatWithHistory(ctx, []chatMessage{
        {Role: "system", Content: systemPrompt},
        {Role: "user", Content: userMessage},
    })
}
```

### Seed Template Data Structure
```go
// Source: Based on internal/models/template.go + migration 005 schema (verified)
type seedTemplate struct {
    ID          string
    Name        string
    Description string
    Version     int
    Steps       string // JSON
    Variables   string // JSON
    IsActive    bool
}

var demoTemplates = []seedTemplate{
    {
        ID:          "tmpl-code-review",
        Name:        "Code Review Pipeline",
        Description: "Multi-agent code review with security, performance, and maintainability checks",
        Version:     3,
        Steps: `[
            {"id":"s1","instruction_template":"Review code for security vulnerabilities: {{repo_url}}","depends_on":[]},
            {"id":"s2","instruction_template":"Analyze performance implications of changes in {{repo_url}}","depends_on":[]},
            {"id":"s3","instruction_template":"Synthesize review findings from security and performance analysis","depends_on":["s1","s2"]}
        ]`,
        Variables: `[{"name":"repo_url","type":"string","description":"Repository URL to review"}]`,
        IsActive: true,
    },
    // ... 4 more templates
}
```

### Policy Seed Data Structure
```go
// Source: Based on internal/policy/engine.go PolicyRule struct (verified)
var demoPolicies = []struct {
    ID       string
    Name     string
    Rules    string // JSON matching PolicyRule struct
    Priority int
    IsActive bool
}{
    {
        ID:   "pol-security-review",
        Name: "Security Review Required",
        Rules: `{
            "when":{"task_contains":["security","vulnerability","auth","password"]},
            "require":{"agent_skills":["security_analysis"]},
            "max_execution_time_minutes":30
        }`,
        Priority: 100,
        IsActive: true,
    },
    // ... 3-4 more policies
}
```

### Analytics Dashboard with Filters
```go
// Source: Based on internal/handlers/analytics.go GetDashboard (verified)
func (h *AnalyticsHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    rangeParam := r.URL.Query().Get("range")  // "7d", "30d", "all"
    statusParam := r.URL.Query().Get("status") // "completed", "failed", "all"

    // Build WHERE conditions
    var conditions []string
    var args []any
    argN := 1

    if tc := timeCondition(rangeParam); tc != "" {
        conditions = append(conditions, tc)
    }
    if statusParam != "" && statusParam != "all" {
        conditions = append(conditions, fmt.Sprintf("status = $%d", argN))
        args = append(args, statusParam)
        argN++
    }

    where := ""
    if len(conditions) > 0 {
        where = "WHERE " + strings.Join(conditions, " AND ")
    }

    // Apply `where` to ALL queries...
}
```

### Audit Log Time Range Addition
```go
// Source: Based on internal/handlers/auditlog.go List (verified, lines 37-55)
// Add to existing filter builder:
since := r.URL.Query().Get("since")
switch since {
case "1h":
    conditions = append(conditions, "created_at > NOW() - INTERVAL '1 hour'")
case "today":
    conditions = append(conditions, "created_at >= DATE_TRUNC('day', NOW())")
case "7d":
    conditions = append(conditions, "created_at > NOW() - INTERVAL '7 days'")
case "30d":
    conditions = append(conditions, "created_at > NOW() - INTERVAL '30 days'")
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `claude` CLI exec for LLM | Direct OpenAI HTTP API | This phase | Removes `claude` binary dependency; `OPENAI_API_KEY` becomes required |
| Empty manage pages | Seeded demo data | This phase | Templates, policies, analytics, and audit pages show populated content |
| Unfiltered analytics | Time + status filtered analytics | This phase | Dashboard becomes actionable with drill-down capability |
| Unfiltered audit log | Time-ranged audit log | This phase | Audit log supports temporal investigation |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Reusing existing raw HTTP client pattern instead of official OpenAI Go SDK is preferred | Standard Stack / Architecture | LOW -- if official SDK is preferred, the interface is the same (system prompt + user message -> string response); swap is trivial |
| A2 | `gpt-4o-mini` handles JSON-only output well enough without `response_format` parameter | Anti-Patterns | LOW -- if it doesn't, adding `response_format: {"type": "json_object"}` to the request body is a 2-line change |
| A3 | `stripMarkdownFences` is sufficient for OpenAI model output cleanup | Pitfall 6 | LOW -- same cleanup function works across providers |
| A4 | Demo seed script should create static data rather than live-execute tasks via agents | Pitfall 5 | MEDIUM -- if D-09 strictly means live execution, agents must be running for `make seed-demo`. Clarify with user if uncertain |
| A5 | Template usage stats should be computed from `template_executions` join rather than denormalized columns | Pitfall 4 | LOW -- join is simple and avoids sync issues; performance is fine for small datasets |

## Open Questions

1. **Demo seed: static vs live execution?**
   - What we know: D-09 says "creates and executes several tasks through the platform", D-10 says "generating real analytics and audit data"
   - What's unclear: Whether this means calling the API while agents are running, or inserting pre-built completed task data directly into the database
   - Recommendation: Implement a hybrid approach -- seed templates and policies as static data (always available), and provide a demo script that can optionally execute live tasks when agents are running. Alternatively, seed completed tasks with pre-built subtask/event data directly in the database to guarantee the dashboard looks populated regardless of agent availability.

2. **Template usage stats: new field or computed?**
   - What we know: D-04 requires "usage stats (usage_count, success_rate, avg_duration)" on templates
   - What's unclear: Whether to add columns to `workflow_templates` or compute from `template_executions`
   - Recommendation: Compute from `template_executions` via a LEFT JOIN in the list query. This avoids denormalization and uses the existing schema. For seeded data, insert corresponding `template_executions` records alongside templates.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| PostgreSQL | All seed data, analytics queries | Expected | 12+ | None -- required |
| OPENAI_API_KEY env var | Task orchestration (D-01) | User must configure | -- | None -- required for task execution |
| Go | Backend compilation | Expected | 1.26.1 | None -- required |
| Node.js / pnpm | Frontend build | Expected | 22.x | None -- required |

No new external tools required. All dependencies are already in use.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | Phase does not change auth |
| V3 Session Management | No | Phase does not change sessions |
| V4 Access Control | No | Phase does not change access control |
| V5 Input Validation | Yes | Validate `range`, `status`, `since` query params against allowlists -- never interpolate raw user input into SQL |
| V6 Cryptography | No | Phase does not change crypto |

### Known Threat Patterns for this Phase

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SQL injection via filter params | Tampering | Validate against allowlist of known values (e.g., "7d", "30d", "all"); never use string interpolation for time intervals in SQL |
| API key exposure in logs | Information Disclosure | Never log the OPENAI_API_KEY value; only log that it is present/missing |
| Seed data in production | Information Disclosure | Seed functions should only run in local mode (check `cfg.IsLocal()`) |

## Sources

### Primary (HIGH confidence)
- `internal/orchestrator/orchestrator.go` -- callLLM function using claude CLI (lines 134-147)
- `cmd/openaiagent/openai.go` -- Existing OpenAI HTTP client pattern (full file)
- `internal/seed/devseed.go` -- Existing seed pattern with idempotent upsert
- `internal/handlers/analytics.go` -- Current analytics endpoint (full file)
- `internal/handlers/auditlog.go` -- Current audit log with dynamic WHERE builder (lines 37-55)
- `internal/models/template.go` -- WorkflowTemplate, TemplateExecution structs
- `internal/policy/engine.go` -- PolicyRule struct with When/Require/Restrict fields
- `internal/db/migrations/005_remove_org_add_templates.sql` -- Schema for workflow_templates, template_versions, template_executions, policies
- `go.mod` -- Current dependency list (5 direct dependencies)

### Secondary (MEDIUM confidence)
- [OpenAI Go SDK repository](https://github.com/openai/openai-go) -- Official SDK at v3.30.0, import `github.com/openai/openai-go/v3`
- [OpenAI Structured Outputs docs](https://developers.openai.com/api/docs/guides/structured-outputs) -- JSON mode and structured output patterns

### Tertiary (LOW confidence)
- None -- all claims verified against codebase or official sources

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries already in use, no new dependencies
- Architecture: HIGH -- patterns extracted from existing codebase
- Pitfalls: HIGH -- identified from direct code analysis of existing handlers and schema
- Seed data: MEDIUM -- exact content is Claude's discretion; structure verified from schema

**Research date:** 2026-04-06
**Valid until:** 2026-05-06 (stable -- no fast-moving dependencies)
