# TaskHub Meta-Agent Architecture Design

**Date:** 2026-03-27
**Status:** Draft
**Author:** Jasper + Claude

## Overview

TaskHub evolves from an "agent orchestration platform" to a **Meta-Agent** — an agent whose core skill is orchestrating other agents. This architectural shift enables recursive hierarchical delegation via standard A2A protocol, self-evolving workflow templates, and a clear open-source/commercial boundary.

### Core Principle

TaskHub is not a platform that manages agents. **TaskHub IS an agent** that happens to have orchestration capabilities. Its Web UI is a management console for this agent, not the product's primary identity.

### Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Hierarchy structure | Static config | Admin defines org tree; auto-discovery impractical without central gateway |
| Agent capability discovery | Dynamic via A2A | AgentCard fetched from `/.well-known/agent-card.json` |
| TaskHub AgentCard | Auto-aggregated | Skills = union of all connected agents' skills |
| Task decomposition | Hybrid 3-layer | Policy (hard) + Template (optional) + LLM (free) |
| Templates | Self-evolving with user confirmation | Evolve based on execution feedback; never auto-mutate |
| Target users | Engineers + Business users | Engineers configure; business users consume |
| Business model | Open-source core + commercial | Core orchestration open; enterprise features paid |
| Multi-org | Deferred | Remove org concept for now; add back as commercial feature |
| Budget/cost tracking | TaskHub LLM only | Cannot track downstream agent token usage (black box) |

---

## 1. System Architecture

### Dual Identity: A2A Client + Server

```
                    +------------ A2A Server ------------+
                    |  AgentCard (auto-aggregated)        |
                    |  skills: [all downstream skills]    |
  Parent TaskHub -->|                                     |
  or any A2A client |  POST /a2a (JSON-RPC 2.0)          |
                    +----------------+--------------------+
                                     |
                    +----------------v--------------------+
                    |        TaskHub Core Engine           |
                    |                                      |
                    |  +----------+  +-----------------+   |
                    |  | Policy   |  | Template Engine  |   |
                    |  | Engine   |  | (self-evolving)  |   |
                    |  +----+-----+  +-------+---------+   |
                    |       +--------+-------+             |
                    |           +----v----+                |
                    |           | LLM     |                |
                    |           | Decomp  |                |
                    |           +----+----+                |
                    |           +----v----+                |
                    |           | DAG     |                |
                    |           | Executor|                |
                    |           +----+----+                |
                    +----------------+--------------------+
                    +------------ A2A Client -------------+
                    |                                      |
               +----v----+  +----v----+  +-----v--------+
               | Agent A  |  | Agent B  |  | Child        |
               | (review) |  | (analyze)|  | TaskHub      |
               +----------+  +----------+  | (also agent) |
                                            +-------------+
```

### Recursive Hierarchy

Each TaskHub instance exposes itself as a standard A2A agent. A parent TaskHub connects to a child TaskHub using the exact same protocol and logic as connecting to any other agent. The parent does not know or care that the child internally decomposes and delegates.

```
Corp TaskHub (AgentCard: aggregated from all subsidiaries)
  +-- Subsidiary A TaskHub (AgentCard: aggregated from A's agents)
  |     +-- Financial Analysis Agent
  |     +-- Compliance Review Agent
  |     +-- Report Generation Agent
  +-- Subsidiary B TaskHub (AgentCard: aggregated from B's agents)
  |     +-- Market Research Agent
  |     +-- Competitive Analysis Agent
  +-- Standalone Agent (directly connected)
        +-- Translation Agent
```

### Relationship to Existing Code

| Package | Change |
|---------|--------|
| `internal/orchestrator/` | Keep; add Policy and Template inputs to decomposition |
| `internal/executor/` | Keep; add `approval_required` state, skill validation |
| `internal/a2a/` | Keep client; **add server** (JSON-RPC handler, AgentCard aggregation) |
| `internal/handlers/` | Keep REST API; **add A2A JSON-RPC handler** |
| `web/` | Keep; add template management UI, policy UI, A2A server config |

---

## 2. A2A Server — TaskHub as Agent

### AgentCard Endpoint

`GET /.well-known/agent-card.json` — public, no auth required.

```json
{
  "name": "Acme Corp AI Team",
  "description": "Auto-generated: capabilities include financial analysis, compliance review, report generation...",
  "version": "1.0.0",
  "supportedInterfaces": [
    { "url": "https://acme.taskhub.com/a2a", "protocolBinding": "HTTP+JSON" }
  ],
  "capabilities": {
    "streaming": true,
    "pushNotifications": false
  },
  "skills": [
    { "id": "financial-analysis", "name": "Financial Analysis", "tags": ["finance", "analysis"] },
    { "id": "compliance-review", "name": "Compliance Review", "tags": ["compliance", "legal"] },
    { "id": "report-gen", "name": "Report Generation", "tags": ["writing", "document"] }
  ],
  "securitySchemes": { "bearer": { "type": "http", "scheme": "bearer" } }
}
```

### Aggregation Rules

- `skills` = union of all online agents' skills, deduplicated by skill id
- `name` / `description` = admin-configurable override, or LLM-generated summary from skills
- AgentCard **auto-regenerates** when agents come online/offline; cached with `ETag` / `Cache-Control`
- When multiple agents provide the same skill, AgentCard merges into one entry; internal routing selects the best agent per task context

### JSON-RPC Endpoint

`POST /a2a` — JSON-RPC 2.0 handler.

Supported methods:

| Method | Behavior |
|--------|----------|
| `tasks/send` | Receive task -> create internal Task (source="a2a") -> decompose -> execute -> return result as Artifact |
| `tasks/get` | Query task status (maps to internal task) |
| `tasks/cancel` | Cancel task |
| `tasks/sendStreamingMessage` | SSE streaming of execution progress |

### Internal Flow for External Tasks

```
External A2A tasks/send
  -> Create internal Task (source="a2a", caller_task_id=caller's task ID)
  -> Standard Pipeline: Policy -> Template -> LLM decomposition
  -> DAG execution
  -> On completion: package result as A2A Artifact, return to caller
  -> During execution: stream TaskStatusUpdateEvent via SSE
```

### Authentication Model

```
External -> TaskHub:
  -> Validated by a2a_server_config.security_scheme (bearer / API key)

TaskHub -> Downstream Agents:
  -> Uses securitySchemes from each agent's AgentCard
  -> Credentials configured by admin during agent registration, stored encrypted
```

### A2A Client Enhancements

| Enhancement | Description |
|-------------|-------------|
| Streaming support | Use `tasks/sendStreamingMessage` for agents that support it |
| AgentCard caching | Respect `ETag`/`Cache-Control`, periodic refresh |
| Capability drift alerting | Detect when agent skills change; notify admin |
| Health check enhancement | Periodic ping; remove offline agent skills from aggregated card |

---

## 3. Task Decomposition Pipeline

### Three-Layer Pipeline

```
Task Input
  |
  v
+-----------------------------------+
| Layer 1: Policy Engine (hard)     |  <- Admin-configured, always active
| - Routing rules                   |
| - Step count / time limits        |
| - Agent restrictions              |
+----------------+------------------+
                 |
                 v
+-----------------------------------+
| Layer 2: Template Match (skeleton)|  <- Optional, user selects or auto-recommended
| - Match template -> provide steps |
| - No template -> skip to LLM     |
+----------------+------------------+
                 |
                 v
+-----------------------------------+
| Layer 3: LLM Decomposition        |  <- Core decomposition engine
| - Input: task + policy + template |
|          + online agent skills    |
| - Output: SubTask DAG (JSON)      |
+----------------+------------------+
                 |
                 v
          DAG Executor
```

### Layer 1: Policy Engine

Policies are organization-level hard rules. LLM must comply; executor enforces as backstop.

```json
{
  "policies": [
    {
      "name": "compliance-gate",
      "when": { "task_contains": ["financial", "legal"] },
      "require": { "agent_skills": ["compliance-review"] },
      "position": "before_final_step"
    },
    {
      "name": "orchestration-limits",
      "max_subtasks": 20,
      "max_execution_time_minutes": 60,
      "max_replan_attempts": 2,
      "require_approval_above_subtasks": 10
    },
    {
      "name": "data-isolation",
      "when": { "task_contains": ["customer-data", "PII"] },
      "restrict_agents_to": { "tags": ["soc2-certified"] }
    }
  ]
}
```

Policies are injected into the LLM prompt as constraints AND enforced by executor at runtime (dual enforcement — LLM may ignore prompts, executor will not).

### Layer 2: Template Match

When creating a task, the user can:
- **No template** -> pure LLM decomposition
- **Manual selection** -> use template skeleton
- **Auto-recommend** -> system matches task description against templates semantically, suggests "This task is 85% similar to Deal Review template, use it?"

Template data structure:

```json
{
  "id": "tmpl_deal_review_v3",
  "name": "Deal Review",
  "version": 3,
  "steps": [
    {
      "id": "research",
      "requires": { "skills": ["web-search", "data-analysis"], "tags": ["research"] },
      "instruction_template": "Research market data and financial information for {target_company}",
      "variables": ["target_company"]
    },
    {
      "id": "analysis",
      "requires": { "skills": ["financial-analysis"] },
      "instruction_template": "Analyze financial health of {target_company} based on research results",
      "depends_on": ["research"]
    },
    {
      "id": "compliance",
      "requires": { "skills": ["compliance-review"] },
      "depends_on": ["research"],
      "mandatory": true
    },
    {
      "id": "report",
      "requires": { "skills": ["document-generation"] },
      "depends_on": ["analysis", "compliance"]
    }
  ]
}
```

Templates define **capability requirements, not specific agents**. At runtime, the routing layer matches `requires` against online agents' AgentCard skills/tags. If no agent matches, the user is told which capability is missing.

Templates are passed to LLM as a reference skeleton: "Use this as a guide. You may adjust specific instructions, add auxiliary steps, and assign agents based on availability. Steps marked mandatory=true cannot be removed."

### Layer 3: LLM Decomposition

LLM receives:

```
[Task] {user's task description}

[Policy Constraints]
- Must include agent with compliance-review skill (task contains "financial")
- Max 20 subtasks, max 60 minutes

[Template Skeleton] (if applicable)
- research -> analysis -> compliance(mandatory) -> report

[Available Agents]
Agent "FinBot" (online): skills=[financial-analysis, data-analysis], tags=[finance, soc2-certified]
Agent "WebCrawler" (online): skills=[web-search, scraping], tags=[research]
Agent "CompliBot" (online): skills=[compliance-review, legal-analysis], tags=[compliance, soc2-certified]
Agent "DocGen" (online): skills=[document-generation, formatting], tags=[writing]

[Output Format]
Return JSON: subtasks array, each with id, agent_id, instruction, depends_on
```

### Agent-Skill Validation

LLM makes primary assignment decisions. Executor validates as backstop:

```
LLM assigns subtask to agent
  -> Executor checks: does agent's skills cover template step's requires?
  -> Mismatch -> reject, request LLM reassignment
  -> Match -> execute
```

When multiple agents satisfy the same skill, LLM chooses based on task context. For deterministic control, Policy can specify agent priority.

### Cost Controls

TaskHub can only track and control its own LLM costs (decomposition, replanning). Downstream agent costs are black-box.

What TaskHub controls:
- Own LLM call costs (existing audit_logs)
- Max subtask count
- Max execution time
- Max retry/replan attempts

Optional passive aggregation: if agents voluntarily report cost in A2A artifact metadata (`cost_usd`), TaskHub can aggregate for display — but this is best-effort, not enforceable.

---

## 4. Self-Evolving Template System

### Template Lifecycle

```
Birth -> Use -> Observe -> Propose Evolution -> User Confirms -> New Version
  ^                                                               |
  +---------------------------------------------------------------+
```

### Birth Methods

**Method 1: Save from successful task**

After task completion, user clicks "Save as Template." System extracts skeleton from actual DAG (steps, dependencies, capability requirements), strips specific agent IDs, retains skill requirements.

**Method 2: System proactive suggestion**

System detects similar task patterns recurring (3+ times), proactively suggests:
"Your last 3 tasks followed a 'research -> analysis -> report' pattern. Save as template?"

Similarity detection: structural comparison of step skill-requirement sequences and dependency graphs (not natural language similarity — more stable).

### Feedback Signal Collection

Recorded automatically on every template-based execution:

| Signal | Meaning |
|--------|---------|
| Step success/failure rate | Repeatedly failing step -> may need splitting or validation pre-step |
| User HITL interventions | Frequent intervention at a step -> needs guardrail or instruction change |
| Replan triggers | Step always triggers replan -> dependency issue |
| User-added/removed steps | User repeatedly adds same step -> should be in template |
| Duration anomalies | Step takes much longer than average -> consider parallel split |
| LLM deviation from template | LLM decomposition diverges from template -> template may be stale |

### Evolution Mechanism

System generates evolution proposals; **user confirms before version upgrade**. Never auto-mutate.

```
Template Evolution Proposal

Template: Deal Review (v2)
Based on analysis of last 5 executions:

Suggestion 1: + Add "data-validation" step between research and analysis
  Reason: 3/5 executions, user manually added this step

Suggestion 2: ~ Make compliance parallel with analysis (remove dependency)
  Reason: No actual data dependency; saves time

Suggestion 3: ~ Update research step instruction
  Reason: LLM deviated from original instruction in 5 consecutive runs

        [Accept All]  [Review Each]  [Dismiss]
```

### Version Management

```json
{
  "id": "tmpl_deal_review",
  "current_version": 3,
  "versions": [
    {
      "version": 1,
      "created_at": "2026-03-20",
      "source": "manual_save",
      "from_task_id": "task_abc123"
    },
    {
      "version": 2,
      "created_at": "2026-03-23",
      "source": "evolution",
      "changes": ["added compliance step"],
      "based_on_executions": 3
    },
    {
      "version": 3,
      "created_at": "2026-03-27",
      "source": "evolution",
      "changes": ["parallelized compliance + analysis", "updated research instruction"],
      "based_on_executions": 5,
      "success_rate_improvement": "78% -> 92%"
    }
  ]
}
```

Users can rollback to any historical version.

### Evolution Triggers

Not analyzed after every execution — thresholds:
- Every **5 uses** of the same template triggers analysis
- Or when a single execution has **severe deviation** (failure, heavy HITL) triggers immediate analysis
- Analysis performed by LLM: input = template + last N execution records, output = evolution suggestions

### Open-Source / Commercial Boundary

- **Open-source**: Save template from task, manual version management, template selection
- **Commercial**: Auto-evolution proposals, feedback signal analysis, similar task detection and auto-recommendation, version comparison and success rate tracking

---

## 5. Observability, Audit & HITL

### Observability Layers

```
Layer 1: Task Trace (task granularity)
  Full trajectory from creation to completion

Layer 2: Subtask Trace (step granularity)
  Each subtask's A2A request/response, retries, state transitions

Layer 3: Orchestration Trace (decomposition granularity)
  TaskHub's own LLM calls: decomposition decisions, template matching, replanning
```

### Task Trace — Global View

```
Task: "Analyze Acme Corp Acquisition"
Status: completed
Duration: 12m 34s
Template: Deal Review v3
Subtasks: 4/4 completed

Timeline:
00:00  Task created
00:02  Orchestration: decomposed into 4 subtasks (LLM: 1.2k tokens)
00:03  Template matched: Deal Review v3 (85% similarity)
00:05  [research] -> WebCrawler agent started
02:30  [research] -> completed
02:31  [analysis] -> FinBot agent started
02:32  [compliance] -> CompliBot agent started (parallel)
06:45  [analysis] -> completed
07:20  [compliance] -> completed
07:21  [report] -> DocGen agent started
12:34  [report] -> completed
12:34  Task completed
```

### Subtask Trace — A2A Interaction Detail

Per-subtask expandable view showing A2A request/response payloads, attempt count, duration, A2A task ID. Only protocol-level data is recorded — agent internal behavior is not tracked (we don't host agents).

### Orchestration Trace — Decomposition Audit

Enhanced audit_logs for TaskHub's own LLM calls:

| Field | Description |
|-------|-------------|
| `action` | `decompose` / `replan` / `template_match` / `evolution_analyze` |
| `input_summary` | Summary of LLM input (not full text, avoid data bloat) |
| `output` | LLM's decomposition result |
| `tokens_in` / `tokens_out` | Token consumption |
| `cost_usd` | Cost |
| `latency_ms` | Duration |
| `policy_applied` | Which policies were injected |
| `template_used` | Which template was used (if any) |

### Audit Log

Immutable records for compliance/security teams. Each record tracks WHO, WHAT, WHEN, HOW, and RESULT. Records cannot be modified or deleted. Query supports filtering by time, user, agent, task.

Uses existing `events` table with new event types rather than a separate audit table.

### Human-in-the-Loop

Three HITL modes:

**Mode 1: Agent-initiated (existing — input_required)**

Agent returns A2A `state: input_required`. DAG pauses. User replies in chat. System sends follow-up to agent via A2A. Existing logic preserved.

**Mode 2: Policy-initiated (new — approval_required)**

Policy rule triggers approval gate:
```
Task decomposed into 15 subtasks
  -> Policy "require_approval_above_subtasks: 10" triggers
  -> Task pauses, status = "approval_required"
  -> User notified
  -> User sees decomposition plan in UI: [Approve] / [Modify] / [Reject]
  -> Approved: continue execution
```

**Mode 3: User-initiated (new — manual intervention)**

User can at any time on the DAG view:
- **Pause** a running task
- Review current state, choose [Continue] / [Cancel] / [Modify pending steps]
- Edit instructions or reassign agents for not-yet-executed subtasks

### Human-on-the-Loop (Default)

Most tasks don't need per-step approval. Default behavior:
- Task executes silently, progress streamed to UI in real-time
- User notified only on exceptions (failure, retry exhausted, agent offline)
- User can check in at any time but is not forced to

Only Policy triggers or agent `input_required` cause the task to pause and wait.

### Open-Source / Commercial Boundary

- **Open-source**: Task/Subtask trace, A2A interaction logs, basic HITL (input_required + pause/continue), event log
- **Commercial**: Orchestration trace (LLM decision audit), immutable audit log, policy-triggered approvals, advanced filtering/export, compliance report generation

---

## 6. Data Model Changes

### New Tables

#### `workflow_templates`

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | PK |
| `name` | TEXT | Template name |
| `description` | TEXT | Template description |
| `version` | INT | Current version number |
| `steps` | JSONB | Step definitions (requires, depends_on, instruction_template, mandatory) |
| `variables` | JSONB | Template variable definitions |
| `source_task_id` | UUID | Task this was born from (nullable) |
| `is_active` | BOOL | Enabled |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

#### `template_versions`

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | PK |
| `template_id` | UUID | FK -> workflow_templates |
| `version` | INT | Version number |
| `steps` | JSONB | Step snapshot for this version |
| `source` | TEXT | `manual_save` / `evolution` / `user_edit` |
| `changes` | JSONB | Change description array |
| `based_on_executions` | INT | How many executions informed this version |
| `created_at` | TIMESTAMPTZ | |

#### `policies`

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | PK |
| `name` | TEXT | Policy name |
| `rules` | JSONB | Rule definitions (when conditions + require/restrict actions) |
| `priority` | INT | Priority for conflict resolution |
| `is_active` | BOOL | |
| `created_at` | TIMESTAMPTZ | |

#### `template_executions`

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | PK |
| `template_id` | UUID | FK -> workflow_templates |
| `template_version` | INT | Version used |
| `task_id` | UUID | FK -> tasks |
| `actual_steps` | JSONB | Steps actually executed (diff from template) |
| `hitl_interventions` | INT | User intervention count |
| `replan_count` | INT | Replan count |
| `outcome` | TEXT | `completed` / `failed` / `canceled` |
| `duration_seconds` | INT | |
| `created_at` | TIMESTAMPTZ | |

### Modified Tables

#### `tasks` — new columns

| Column | Type | Description |
|--------|------|-------------|
| `source` | TEXT | `web` / `a2a` / `api` |
| `caller_task_id` | TEXT | Caller's A2A task ID when source=a2a |
| `template_id` | UUID | Template used (nullable) |
| `template_version` | INT | Template version used |
| `policy_applied` | JSONB | Policies that were applied |

#### `subtasks` — new columns

| Column | Type | Description |
|--------|------|-------------|
| `status` | — | Add `approval_required` state |
| `matched_skills` | JSONB | Skills matched when assigning agent |
| `attempt_history` | JSONB | Summary of each retry attempt |

#### `agents` — new columns

| Column | Type | Description |
|--------|------|-------------|
| `is_online` | BOOL | Health check status |
| `last_health_check` | TIMESTAMPTZ | Last health check time |
| `skill_hash` | TEXT | Hash of AgentCard skills for drift detection |

### New Configuration

#### `a2a_server_config` (singleton)

| Column | Type | Description |
|--------|------|-------------|
| `id` | INT | PK (always 1) |
| `enabled` | BOOL | A2A Server enabled |
| `name_override` | TEXT | Custom AgentCard name (nullable) |
| `description_override` | TEXT | Custom description (nullable) |
| `security_scheme` | JSONB | Auth configuration |
| `aggregated_card` | JSONB | Cached aggregated AgentCard |
| `card_updated_at` | TIMESTAMPTZ | Last aggregation time |

### Removed Tables

- `organizations` — deferred to commercial version
- `org_members` — deferred to commercial version

All `org_id` foreign keys removed from existing tables.

---

## 7. API Changes

### New: A2A Server Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/.well-known/agent-card.json` | Aggregated AgentCard (public, no auth) |
| POST | `/a2a` | JSON-RPC 2.0 entry (tasks/send, tasks/get, tasks/cancel, tasks/sendStreamingMessage) |

`/a2a` endpoint auth controlled by `a2a_server_config.security_scheme`, independent from Web UI auth.

### New: Template Management API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/templates` | List all templates |
| POST | `/api/templates` | Create template (manual) |
| GET | `/api/templates/{id}` | Get template detail (with version history) |
| PUT | `/api/templates/{id}` | Update template |
| DELETE | `/api/templates/{id}` | Delete template |
| POST | `/api/templates/from-task/{task_id}` | Create template from completed task |
| POST | `/api/templates/{id}/rollback/{version}` | Rollback to version |
| GET | `/api/templates/{id}/executions` | View template execution records |
| POST | `/api/templates/{id}/match` | Check task description match score |

### New: Policy Management API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/policies` | List all policies |
| POST | `/api/policies` | Create policy |
| PUT | `/api/policies/{id}` | Update policy |
| DELETE | `/api/policies/{id}` | Delete policy |

### New: A2A Server Config API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/a2a-config` | Get A2A Server configuration |
| PUT | `/api/a2a-config` | Update configuration |
| POST | `/api/a2a-config/refresh-card` | Manually trigger AgentCard re-aggregation |

### Modified: Task Creation

`POST /api/tasks` — new request body fields:

```json
{
  "title": "Analyze Acme Corp acquisition",
  "description": "...",
  "template_id": "tmpl_xxx",
  "template_variables": {
    "target_company": "Acme Corp"
  }
}
```

### Modified: Task Detail Response

`GET /api/tasks/{id}` — new response fields:

```json
{
  "source": "web",
  "template_id": "tmpl_xxx",
  "template_version": 3,
  "policy_applied": ["compliance-gate"],
  "subtasks": [
    {
      "matched_skills": ["compliance-review"],
      "attempt_history": [...]
    }
  ]
}
```

### Modified: Agent List Response

`GET /api/agents` — new response fields:

```json
{
  "is_online": true,
  "last_health_check": "2026-03-27T10:00:00Z",
  "skill_drift_detected": false
}
```

### New SSE Event Types

| Event Type | Trigger |
|------------|---------|
| `policy.applied` | Policy rule matched |
| `approval.requested` | Policy-triggered approval pause |
| `approval.resolved` | User approved/rejected |
| `template.matched` | Auto-matched template |
| `template.deviated` | LLM decomposition deviated from template |
| `agent.offline` | Agent health check failed |
| `agent.skill_drift` | Agent capability changed |

### New Frontend Pages

| Page | Path | Description |
|------|------|-------------|
| Template list | `/templates` | View, search, manage templates |
| Template detail | `/templates/[id]` | Step visualization, version history, execution stats, evolution suggestions |
| Policy management | `/settings/policies` | CRUD for policies |
| A2A Server config | `/settings/a2a-server` | Toggle, AgentCard preview, auth config |
| Task creation | (existing dialog) | Add template selector and variable input |

---

## 8. Open-Source / Commercial Boundary

### Open-Source Core

Everything needed to run TaskHub as a functional meta-agent:

- A2A client + server (full protocol support)
- AgentCard auto-aggregation
- LLM task decomposition
- DAG execution engine (dependencies, retries, replanning, crash recovery)
- Basic HITL (agent input_required + user pause/continue)
- Template CRUD (create, save from task, manual version management, select on task creation)
- Policy engine (basic routing rules, step/time limits)
- Task/Subtask trace (timeline, A2A interaction logs)
- SSE real-time streaming
- Web UI (dashboard, DAG view, chat, agent management)
- Single-user / single-instance deployment

### Commercial Features

Enterprise capabilities layered on top:

- **Self-evolving templates**: feedback analysis, evolution proposals, auto-recommendation, success rate tracking
- **Advanced audit**: orchestration trace (LLM decision audit), immutable logs, compliance reports, export
- **Policy-triggered HITL**: approval gates, require_approval_above_subtasks
- **Multi-organization**: org isolation, org-scoped RBAC, org-scoped policies
- **SSO / advanced auth**: OIDC, SAML
- **Agent analytics**: performance leaderboard, capability drift monitoring, online/offline history
- **Template marketplace**: share templates across teams/orgs
- **Advanced DAG features**: conditional branching, sub-graph composition

---

## Appendix: Competitive Research Summary

Key ideas incorporated from competitive analysis:

| Source | Idea | How Applied |
|--------|------|-------------|
| CrewAI Flows | Deterministic backbone + agentic nodes | Template as skeleton, LLM fills in |
| CrewAI Guardrails | Task-level validation | Policy engine dual enforcement |
| LangGraph | Checkpoint-based HITL | Pause/resume at any point, state persisted |
| LangGraph | Sub-graph composition | Child TaskHub = sub-graph (via A2A) |
| OpenAI Agents SDK | Agent-as-tool | TaskHub-as-agent (recursive) |
| Semantic Kernel | Unified orchestration API | Template defines pattern, agents are parameters |
| Letta (MemGPT) | Memory tiering | Future: cross-session state for long workflows |
| Relevance AI | Workforce metaphor | UI presents agents as team members |
| Agent Mesh (Solo.io) | Capability drift detection | Agent skill_hash comparison |
| A2A v0.3 | Agent Card signing | Future: Ed25519 signature verification |
| Kubernetes | Requirements-driven scheduling | Template defines skill requirements, runtime matches to agents |
