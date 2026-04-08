# Plan 1: Foundation — Remove Org Concept + New Data Model

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Strip the organization multi-tenancy layer and add new tables (templates, policies, a2a_server_config) to prepare the codebase for the Meta-Agent architecture.

**Architecture:** Bottom-up approach. First add new tables and model structs (non-breaking). Then update each consumer one at a time (each step compiles and can be tested). Only remove old org fields from shared models after all consumers are updated. Frontend updated last.

**Tech Stack:** Go (chi router, pgx/v5), PostgreSQL, Next.js 15, TypeScript, Zustand

**Key constraint:** Every task must end with `go build ./...` (or `npm run build` for frontend tasks) passing. No task may leave the codebase in a broken state.

---

### Task 1: Create feature branch + write migration SQL

**Files:**
- Create: `internal/db/migrations/005_remove_org_add_templates.sql`
- Modify: `internal/db/migrate.go`

- [ ] **Step 1: Create feature branch**

```bash
git checkout -b feat/meta-agent-foundation
```

- [ ] **Step 2: Write migration SQL**

Create `internal/db/migrations/005_remove_org_add_templates.sql`:

```sql
-- 005: Remove org concept + add template/policy/a2a-config tables

-- ============================================================
-- PART 1: Remove org-related constraints and tables
-- ============================================================

DROP TABLE IF EXISTS agent_role_permissions;
DROP TABLE IF EXISTS agent_user_permissions;
DROP TABLE IF EXISTS org_members;

-- audit_logs: drop org_id column
ALTER TABLE audit_logs DROP COLUMN IF EXISTS org_id;

-- agents: drop org_id and the unique constraint on (org_id, name)
ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_org_id_name_key;
ALTER TABLE agents DROP COLUMN IF EXISTS org_id;
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'agents_name_key'
  ) THEN
    ALTER TABLE agents ADD CONSTRAINT agents_name_key UNIQUE (name);
  END IF;
END$$;

-- tasks: drop org_id column and its indexes
DROP INDEX IF EXISTS idx_tasks_org_created;
DROP INDEX IF EXISTS idx_tasks_org_status;
ALTER TABLE tasks DROP COLUMN IF EXISTS org_id;
CREATE INDEX IF NOT EXISTS idx_tasks_created ON tasks(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);

-- Drop organizations table
DROP TABLE IF EXISTS organizations;

-- ============================================================
-- PART 2: Add new columns to existing tables
-- ============================================================

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'web';
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS caller_task_id TEXT NOT NULL DEFAULT '';
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS template_id TEXT NOT NULL DEFAULT '';
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS template_version INT NOT NULL DEFAULT 0;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS policy_applied JSONB NOT NULL DEFAULT '[]';

ALTER TABLE subtasks ADD COLUMN IF NOT EXISTS matched_skills JSONB NOT NULL DEFAULT '[]';
ALTER TABLE subtasks ADD COLUMN IF NOT EXISTS attempt_history JSONB NOT NULL DEFAULT '[]';

ALTER TABLE agents ADD COLUMN IF NOT EXISTS is_online BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS last_health_check TIMESTAMPTZ;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS skill_hash TEXT NOT NULL DEFAULT '';

-- ============================================================
-- PART 3: New tables
-- ============================================================

CREATE TABLE IF NOT EXISTS workflow_templates (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL UNIQUE,
    description    TEXT NOT NULL DEFAULT '',
    version        INT NOT NULL DEFAULT 1,
    steps          JSONB NOT NULL DEFAULT '[]',
    variables      JSONB NOT NULL DEFAULT '[]',
    source_task_id TEXT,
    is_active      BOOLEAN NOT NULL DEFAULT true,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_templates_active ON workflow_templates(is_active, created_at DESC);

CREATE TABLE IF NOT EXISTS template_versions (
    id                  TEXT PRIMARY KEY,
    template_id         TEXT NOT NULL REFERENCES workflow_templates(id) ON DELETE CASCADE,
    version             INT NOT NULL,
    steps               JSONB NOT NULL DEFAULT '[]',
    source              TEXT NOT NULL DEFAULT 'manual_save',
    changes             JSONB NOT NULL DEFAULT '[]',
    based_on_executions INT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (template_id, version)
);
CREATE INDEX IF NOT EXISTS idx_template_versions_tmpl ON template_versions(template_id, version);

CREATE TABLE IF NOT EXISTS policies (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    rules      JSONB NOT NULL DEFAULT '{}',
    priority   INT NOT NULL DEFAULT 0,
    is_active  BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS template_executions (
    id                TEXT PRIMARY KEY,
    template_id       TEXT NOT NULL REFERENCES workflow_templates(id) ON DELETE CASCADE,
    template_version  INT NOT NULL DEFAULT 1,
    task_id           TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    actual_steps      JSONB NOT NULL DEFAULT '[]',
    hitl_interventions INT NOT NULL DEFAULT 0,
    replan_count      INT NOT NULL DEFAULT 0,
    outcome           TEXT NOT NULL DEFAULT '',
    duration_seconds  INT NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tmpl_exec_template ON template_executions(template_id, created_at DESC);

CREATE TABLE IF NOT EXISTS a2a_server_config (
    id                   INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    enabled              BOOLEAN NOT NULL DEFAULT false,
    name_override        TEXT,
    description_override TEXT,
    security_scheme      JSONB NOT NULL DEFAULT '{}',
    aggregated_card      JSONB NOT NULL DEFAULT '{}',
    card_updated_at      TIMESTAMPTZ
);
INSERT INTO a2a_server_config (id) VALUES (1) ON CONFLICT DO NOTHING;
```

- [ ] **Step 3: Register migration in migrate.go**

In `internal/db/migrate.go`, change the files list:

```go
files := []string{
    "migrations/001_foundation.sql",
    "migrations/004_a2a_migration.sql",
    "migrations/005_remove_org_add_templates.sql",
}
```

- [ ] **Step 4: Verify build**

```bash
go build ./internal/db/...
```

Expected: PASS (migration is embedded SQL, no Go API changes).

- [ ] **Step 5: Commit**

```bash
git add internal/db/migrations/005_remove_org_add_templates.sql internal/db/migrate.go
git commit -m "feat(db): add migration 005 — remove org, add template/policy/a2a tables"
```

---

### Task 2: Add new model structs and fields (non-breaking)

**Files:**
- Modify: `internal/models/agent.go` (add new fields — non-breaking)
- Create: `internal/models/template.go`
- Create: `internal/models/policy.go`

Adding fields to existing structs and creating new files are both non-breaking changes.

- [ ] **Step 1: Add new fields to Agent struct (non-breaking)**

In `internal/models/agent.go`, add three fields before `CreatedAt` (keep `OrgID` — it will be removed later in Task 9):

```go
	Status          string          `json:"status"`
	IsOnline        bool            `json:"is_online"`
	LastHealthCheck *time.Time      `json:"last_health_check,omitempty"`
	SkillHash       string          `json:"skill_hash,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
```

- [ ] **Step 2: Verify build — should still pass**

```bash
go build ./...
```

Expected: PASS — adding struct fields is non-breaking; no existing code reads these fields yet.

- [ ] **Step 3: Create template.go**

Create `internal/models/template.go`:

```go
package models

import (
	"encoding/json"
	"time"
)

type WorkflowTemplate struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Version      int             `json:"version"`
	Steps        json.RawMessage `json:"steps"`
	Variables    json.RawMessage `json:"variables"`
	SourceTaskID string          `json:"source_task_id,omitempty"`
	IsActive     bool            `json:"is_active"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type TemplateVersion struct {
	ID                string          `json:"id"`
	TemplateID        string          `json:"template_id"`
	Version           int             `json:"version"`
	Steps             json.RawMessage `json:"steps"`
	Source            string          `json:"source"`
	Changes           json.RawMessage `json:"changes"`
	BasedOnExecutions int             `json:"based_on_executions"`
	CreatedAt         time.Time       `json:"created_at"`
}

type TemplateExecution struct {
	ID                string          `json:"id"`
	TemplateID        string          `json:"template_id"`
	TemplateVersion   int             `json:"template_version"`
	TaskID            string          `json:"task_id"`
	ActualSteps       json.RawMessage `json:"actual_steps"`
	HITLInterventions int             `json:"hitl_interventions"`
	ReplanCount       int             `json:"replan_count"`
	Outcome           string          `json:"outcome"`
	DurationSeconds   int             `json:"duration_seconds"`
	CreatedAt         time.Time       `json:"created_at"`
}
```

- [ ] **Step 2: Create policy.go**

Create `internal/models/policy.go`:

```go
package models

import (
	"encoding/json"
	"time"
)

type Policy struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Rules     json.RawMessage `json:"rules"`
	Priority  int             `json:"priority"`
	IsActive  bool            `json:"is_active"`
	CreatedAt time.Time       `json:"created_at"`
}

type A2AServerConfig struct {
	ID                  int             `json:"id"`
	Enabled             bool            `json:"enabled"`
	NameOverride        *string         `json:"name_override,omitempty"`
	DescriptionOverride *string         `json:"description_override,omitempty"`
	SecurityScheme      json.RawMessage `json:"security_scheme"`
	AggregatedCard      json.RawMessage `json:"aggregated_card"`
	CardUpdatedAt       *time.Time      `json:"card_updated_at,omitempty"`
}
```

- [ ] **Step 3: Verify full build**

```bash
go build ./...
```

Expected: PASS — new files, nothing changed.

- [ ] **Step 4: Commit**

```bash
git add internal/models/template.go internal/models/policy.go
git commit -m "feat(models): add template, policy, and a2a-config structs"
```

---

### Task 3: Update executor — remove org budget and change agent loading

Update the executor first because it's a self-contained module that other handlers depend on. After this task, the executor no longer queries the `organizations` table.

**Files:**
- Modify: `internal/executor/executor.go`

- [ ] **Step 1: Write test for loadAgents (no org filter)**

Create `internal/executor/executor_test.go`:

```go
package executor

import "testing"

// TestLoadAgentsSignature verifies the function exists with the correct signature.
// Full integration tests require a database; this just ensures compilation.
func TestLoadAgentsSignature(t *testing.T) {
	var e DAGExecutor
	// loadAgents should accept only context, not orgID
	_ = e.loadAgents // compile-time check
}
```

- [ ] **Step 2: Run test — should fail**

```bash
go test ./internal/executor/ -run TestLoadAgentsSignature -v
```

Expected: FAIL — `loadAgents` doesn't exist yet (it's `loadOrgAgents`).

- [ ] **Step 3: Rename loadOrgAgents to loadAgents, remove org filter**

In `internal/executor/executor.go`, find the `loadOrgAgents` function (around line 928) and replace:

```go
func (e *DAGExecutor) loadOrgAgents(ctx context.Context, orgID string) ([]models.Agent, error) {
	rows, err := e.DB.Query(ctx,
		`SELECT id, org_id, name, COALESCE(version,''), COALESCE(description,''), endpoint,
		        COALESCE(agent_card_url,''), COALESCE(agent_card,'{}'), card_fetched_at,
		        COALESCE(capabilities, '[]'), COALESCE(skills,'[]'),
		        COALESCE(status,'active'), created_at, updated_at
		 FROM agents WHERE org_id = $1 AND status = 'active'`, orgID)
```

With:

```go
func (e *DAGExecutor) loadAgents(ctx context.Context) ([]models.Agent, error) {
	rows, err := e.DB.Query(ctx,
		`SELECT id, name, COALESCE(version,''), COALESCE(description,''), endpoint,
		        COALESCE(agent_card_url,''), COALESCE(agent_card,'{}'), card_fetched_at,
		        COALESCE(capabilities, '[]'), COALESCE(skills,'[]'),
		        COALESCE(status,'active'), created_at, updated_at
		 FROM agents WHERE status = 'active'`)
```

In the row scanning inside this function, remove `&a.OrgID`:

Replace: `&a.ID, &a.OrgID, &a.Name,`
With: `&a.ID, &a.Name,`

- [ ] **Step 4: Update all call sites of loadOrgAgents**

Search for `loadOrgAgents` in `executor.go` and replace all calls:

Replace: `e.loadOrgAgents(ctx, task.OrgID)` or similar
With: `e.loadAgents(ctx)`

There should be 2-3 call sites (in `Execute`, and in the replan path).

- [ ] **Step 5: Remove org budget functions**

Delete these functions entirely from `executor.go`:
- `checkBudget` (around line 779)
- `checkBudgetForOrg` (around line 806)

Remove `ErrBudgetExceeded` from the var block:

Replace:
```go
var (
	ErrBudgetExceeded = errors.New("monthly budget exceeded")
	ErrTaskCanceled   = errors.New("task was canceled")
)
```

With:
```go
var ErrTaskCanceled = errors.New("task was canceled")
```

- [ ] **Step 6: Update Execute function — remove budget check**

In the `Execute` function, replace the budget + agent loading block:

Replace:
```go
	// 2. Load org for budget checks
	var orgBudget *float64
	var orgAlertThreshold float64
	err := e.DB.QueryRow(ctx,
		`SELECT budget_usd_monthly, budget_alert_threshold FROM organizations WHERE id = $1`,
		task.OrgID).Scan(&orgBudget, &orgAlertThreshold)
	if err != nil {
		return fmt.Errorf("load org: %w", err)
	}

	// 3. Check budget before LLM call
	if err := e.checkBudget(ctx, task.OrgID, orgBudget); err != nil {
		_ = e.updateTaskStatus(ctx, task.ID, "failed", err.Error())
		e.publishEvent(ctx, task.ID, "", "task.failed", "system", "", map[string]any{"error": err.Error()})
		return err
	}

	// 4. Load agents for this org
	agents, err := e.loadOrgAgents(ctx, task.OrgID)
```

With:
```go
	// 2. Load all active agents
	agents, err := e.loadAgents(ctx)
```

Replace:
```go
		errMsg := "no agents available for this organization"
```

With:
```go
		errMsg := "no active agents available"
```

- [ ] **Step 7: Remove any remaining checkBudgetForOrg calls**

Search the entire file for `checkBudget` — there may be calls in the DAG loop for per-subtask budget checks. Remove them all.

- [ ] **Step 8: Remove unused imports**

If `pgx` import was only used by `checkBudgetForOrg`, remove `"github.com/jackc/pgx/v5"`. Check if `errors` is still used (for `ErrTaskCanceled`).

- [ ] **Step 9: Verify build**

```bash
go build ./...
```

Expected: PASS — executor no longer references org tables. The `Task.OrgID` field still exists in the model but is simply unused by the executor now.

- [ ] **Step 10: Run test**

```bash
go test ./internal/executor/ -v
```

Expected: PASS.

- [ ] **Step 11: Commit**

```bash
git add internal/executor/
git commit -m "refactor(executor): remove org budget checks, load all active agents"
```

---

### Task 4: Simplify ctxutil and RBAC (remove org context)

**Files:**
- Modify: `internal/ctxutil/ctxutil.go`
- Modify: `internal/rbac/middleware.go`
- Modify: `internal/auth/middleware.go`

- [ ] **Step 1: Keep old ctxutil functions but add new ones**

To avoid breaking existing consumers, ADD new functions alongside old ones. In `internal/ctxutil/ctxutil.go`:

Add at the bottom (keeping existing functions):

```go
// RoleFromCtx returns the user's role from context (non-org-scoped).
func RoleFromCtx(ctx context.Context) string {
	// First try new key, then fall back to old org role key for compatibility
	if r, ok := ctx.Value(ctxKeyRole).(string); ok {
		return r
	}
	return OrgRoleFromCtx(ctx)
}

func SetRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, ctxKeyRole, role)
}
```

And add the new context key:

```go
const (
	ctxKeyUser    contextKey = "user"
	ctxKeyOrg     contextKey = "org"
	ctxKeyOrgRole contextKey = "org_role"
	ctxKeyRole    contextKey = "role"
)
```

- [ ] **Step 2: Update auth middleware to set role**

In `internal/auth/middleware.go`, update local mode:

Replace:
```go
		if m.LocalMode {
			ctx := ctxutil.SetUser(r.Context(), localUser)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
```

With:
```go
		if m.LocalMode {
			ctx := ctxutil.SetUser(r.Context(), localUser)
			ctx = ctxutil.SetRole(ctx, "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
```

Update cloud mode similarly:

Replace:
```go
		ctx := ctxutil.SetUser(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
```

With:
```go
		ctx := ctxutil.SetUser(r.Context(), user)
		ctx = ctxutil.SetRole(ctx, "admin")
		next.ServeHTTP(w, r.WithContext(ctx))
```

- [ ] **Step 3: Verify build**

```bash
go build ./...
```

Expected: PASS — old functions still exist, new functions added, no consumers broken.

- [ ] **Step 4: Run existing tests**

```bash
go test ./...
```

Expected: PASS — RBAC roles test unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/ctxutil/ internal/auth/ internal/rbac/
git commit -m "refactor(auth): add non-org role context, prepare for org removal"
```

---

### Task 5: Update task handlers — stop using org context

**Files:**
- Modify: `internal/handlers/tasks.go`

- [ ] **Step 1: Update Create handler — stop using org**

In `Create`, replace:

```go
	org := ctxutil.OrgFromCtx(r.Context())
	user := ctxutil.UserFromCtx(r.Context())

	now := time.Now().UTC()
	task := models.Task{
		ID:          uuid.New().String(),
		OrgID:       org.ID,
		Title:       req.Title,
		Description: strings.TrimSpace(req.Description),
		Status:      "pending",
		CreatedBy:   user.ID,
		ReplanCount: 0,
		CreatedAt:   now,
	}

	_, err := h.DB.Exec(r.Context(),
		`INSERT INTO tasks (id, org_id, title, description, status, created_by, replan_count, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		task.ID, task.OrgID, task.Title, task.Description, task.Status, task.CreatedBy, task.ReplanCount, task.CreatedAt)
```

With:

```go
	user := ctxutil.UserFromCtx(r.Context())

	now := time.Now().UTC()
	task := models.Task{
		ID:          uuid.New().String(),
		OrgID:       "", // deprecated, kept for model compat
		Title:       req.Title,
		Description: strings.TrimSpace(req.Description),
		Status:      "pending",
		CreatedBy:   user.ID,
		ReplanCount: 0,
		CreatedAt:   now,
	}

	_, err := h.DB.Exec(r.Context(),
		`INSERT INTO tasks (id, title, description, status, created_by, replan_count, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		task.ID, task.Title, task.Description, task.Status, task.CreatedBy, task.ReplanCount, task.CreatedAt)
```

- [ ] **Step 2: Update List handler**

Replace:

```go
func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	org := ctxutil.OrgFromCtx(r.Context())
	statusFilter := r.URL.Query().Get("status")
	// ... queries with WHERE org_id = $1
```

With:

```go
func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")

	var (
		rows pgx.Rows
		err  error
	)

	if statusFilter != "" {
		rows, err = h.DB.Query(r.Context(),
			`SELECT id, title, description, status, created_by,
				metadata, plan, result, error, replan_count, created_at, completed_at
			 FROM tasks
			 WHERE status = $1
			 ORDER BY created_at DESC`,
			statusFilter)
	} else {
		rows, err = h.DB.Query(r.Context(),
			`SELECT id, title, description, status, created_by,
				metadata, plan, result, error, replan_count, created_at, completed_at
			 FROM tasks
			 ORDER BY created_at DESC`)
	}
```

- [ ] **Step 3: Update Get handler**

Replace:

```go
	org := ctxutil.OrgFromCtx(r.Context())
	id := chi.URLParam(r, "id")

	task, err := scanTask(
		h.DB.QueryRow(r.Context(),
			`SELECT id, org_id, title, ...
			 WHERE id = $1 AND org_id = $2`, id, org.ID).Scan,
	)
```

With:

```go
	id := chi.URLParam(r, "id")

	task, err := scanTask(
		h.DB.QueryRow(r.Context(),
			`SELECT id, title, description, status, created_by,
				metadata, plan, result, error, replan_count, created_at, completed_at
			 FROM tasks
			 WHERE id = $1`, id).Scan,
	)
```

- [ ] **Step 4: Update Cancel handler**

Replace:

```go
	org := ctxutil.OrgFromCtx(r.Context())
	id := chi.URLParam(r, "id")

	var taskOrgID string
	err := h.DB.QueryRow(r.Context(),
		`SELECT org_id FROM tasks WHERE id = $1`, id).Scan(&taskOrgID)
	if err != nil {
		jsonError(w, "task not found", http.StatusNotFound)
		return
	}
	if taskOrgID != org.ID {
		jsonError(w, "task not found", http.StatusNotFound)
		return
	}
```

With:

```go
	id := chi.URLParam(r, "id")

	var exists bool
	err := h.DB.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM tasks WHERE id = $1)`, id).Scan(&exists)
	if err != nil || !exists {
		jsonError(w, "task not found", http.StatusNotFound)
		return
	}
```

- [ ] **Step 5: Update scanTask — remove OrgID**

Replace:

```go
	err := scan(
		&t.ID, &t.OrgID, &t.Title, &t.Description, &t.Status, &t.CreatedBy,
		&metadata, &plan, &result, &taskError, &t.ReplanCount, &t.CreatedAt, &t.CompletedAt,
	)
```

With:

```go
	err := scan(
		&t.ID, &t.Title, &t.Description, &t.Status, &t.CreatedBy,
		&metadata, &plan, &result, &taskError, &t.ReplanCount, &t.CreatedAt, &t.CompletedAt,
	)
```

Note: `t.OrgID` still exists in the struct, it just won't be populated from DB anymore. This is intentional — we don't modify the model yet.

- [ ] **Step 6: Verify build**

```bash
go build ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/handlers/tasks.go
git commit -m "refactor(handlers): remove org_id from task handlers"
```

---

### Task 6: Update agent handlers — stop using org context

**Files:**
- Modify: `internal/handlers/agents.go`

- [ ] **Step 1: Update agentColumns and scanAgent — remove org_id**

Replace:

```go
const agentColumns = `id, org_id, name, version, description, endpoint,
	agent_card_url, agent_card, card_fetched_at,
	capabilities, skills,
	status, created_at, updated_at`
```

With:

```go
const agentColumns = `id, name, version, description, endpoint,
	agent_card_url, agent_card, card_fetched_at,
	capabilities, skills,
	status, is_online, last_health_check, skill_hash,
	created_at, updated_at`
```

Replace in scanAgent:

```go
	err := scan(
		&a.ID, &a.OrgID, &a.Name, &a.Version, &a.Description, &a.Endpoint,
		&a.AgentCardURL, &agentCard, &a.CardFetchedAt,
		&capabilitiesJSON, &skillsJSON,
		&a.Status, &a.CreatedAt, &a.UpdatedAt,
	)
```

With:

```go
	err := scan(
		&a.ID, &a.Name, &a.Version, &a.Description, &a.Endpoint,
		&a.AgentCardURL, &agentCard, &a.CardFetchedAt,
		&capabilitiesJSON, &skillsJSON,
		&a.Status, &a.IsOnline, &a.LastHealthCheck, &a.SkillHash,
		&a.CreatedAt, &a.UpdatedAt,
	)
```

- [ ] **Step 2: Update List — remove org filter**

Replace:

```go
func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	org := ctxutil.OrgFromCtx(r.Context())

	rows, err := h.DB.Query(r.Context(),
		`SELECT `+agentColumns+`
		 FROM agents
		 WHERE org_id = $1
		 ORDER BY created_at DESC`, org.ID)
```

With:

```go
func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(),
		`SELECT `+agentColumns+`
		 FROM agents
		 ORDER BY created_at DESC`)
```

- [ ] **Step 3: Update Create — remove org**

Remove `org := ctxutil.OrgFromCtx(r.Context())`.

Replace:
```go
	agent := models.Agent{
		ID:            uuid.New().String(),
		OrgID:         org.ID,
		Name:          req.Name,
```

With:
```go
	agent := models.Agent{
		ID:            uuid.New().String(),
		Name:          req.Name,
```

Replace INSERT query — remove `org_id` column and `agent.OrgID` param. Adjust parameter numbering ($1-$13 instead of $1-$14).

```go
	_, err = h.DB.Exec(r.Context(),
		`INSERT INTO agents (id, name, version, description, endpoint,
			agent_card_url, agent_card, card_fetched_at,
			capabilities, skills,
			status, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		agent.ID, agent.Name, agent.Version, agent.Description, agent.Endpoint,
		agent.AgentCardURL, agentCard, cardFetchedAt,
		capsJSON, skills,
		agent.Status, agent.CreatedAt, agent.UpdatedAt)
```

- [ ] **Step 4: Update Get, Update, Delete, Healthcheck — remove org**

For each handler:
1. Remove `org := ctxutil.OrgFromCtx(r.Context())`
2. Change SQL from `WHERE id = $1 AND org_id = $2` to `WHERE id = $1`
3. Remove `org.ID` from query params
4. Adjust parameter numbering in UPDATE queries

- [ ] **Step 5: Remove unused ctxutil import**

Remove `"taskhub/internal/ctxutil"` from the imports.

- [ ] **Step 6: Verify build**

```bash
go build ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/handlers/agents.go
git commit -m "refactor(handlers): remove org_id from agent handlers, add health fields"
```

---

### Task 7: Delete org/member handlers, flatten routes

**Files:**
- Delete: `internal/handlers/orgs.go`
- Delete: `internal/handlers/members.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Delete org and member handler files**

```bash
rm internal/handlers/orgs.go internal/handlers/members.go
```

- [ ] **Step 2: Flatten routes in main.go**

Remove handler initialization:
```go
	orgH := &handlers.OrgHandler{DB: pool}
	memberH := &handlers.MemberHandler{DB: pool}
```

Remove RBAC middleware:
```go
	rbacMw := &rbac.Middleware{DB: pool}
```

Replace the entire API route group with flat routes:

```go
	r.Group(func(r chi.Router) {
		r.Use(authMw.RequireAuth)

		// Agents
		r.Get("/api/agents", agentH.List)
		r.Post("/api/agents", agentH.Create)
		r.Get("/api/agents/{id}", agentH.Get)
		r.Put("/api/agents/{id}", agentH.Update)
		r.Delete("/api/agents/{id}", agentH.Delete)
		r.Post("/api/agents/{id}/healthcheck", agentH.Healthcheck)
		r.Post("/api/agents/test-endpoint", agentH.TestEndpoint)
		r.Post("/api/agents/discover", agentH.Discover)

		// Tasks
		r.Post("/api/tasks", taskH.Create)
		r.Get("/api/tasks", taskH.List)
		r.Get("/api/tasks/{id}", taskH.Get)
		r.Post("/api/tasks/{id}/cancel", taskH.Cancel)
		r.Get("/api/tasks/{id}/cost", taskH.GetCost)
		r.Get("/api/tasks/{id}/subtasks", taskH.ListSubtasks)

		// Messages
		r.Get("/api/tasks/{id}/messages", msgH.List)
		r.Post("/api/tasks/{id}/messages", msgH.Send)

		// SSE
		r.Get("/api/tasks/{id}/events", streamH.Stream)
	})
```

- [ ] **Step 3: Remove unused imports from main.go**

Remove `"taskhub/internal/rbac"` if no longer used. Keep other imports.

- [ ] **Step 4: Update seed if needed**

Check if `seed.LocalSeedAndLog` references orgs. If so, update it to only seed the local user (skip org creation). If the seed file doesn't exist (it wasn't found earlier), skip this step.

- [ ] **Step 5: Verify build**

```bash
go build ./...
```

Expected: may fail if audit package still references org_id. Fix in next task.

- [ ] **Step 6: Commit (if build passes)**

```bash
git add -A
git commit -m "refactor(router): flatten API routes, remove org/member handlers"
```

---

### Task 8: Fix remaining Go compilation errors

**Files:**
- Modify: any file still referencing org (likely `internal/audit/logger.go`)

- [ ] **Step 1: Find all remaining org references**

```bash
go build ./... 2>&1
```

Read the errors. Common remaining issues:
- `internal/audit/logger.go`: `GetOrgMonthlySpend` queries `organizations` table
- Any file using `ctxutil.OrgFromCtx` (message handler doesn't use it, already checked)
- Seed file referencing org

- [ ] **Step 2: Fix each error**

For `GetOrgMonthlySpend`: delete the function if it's only called from the now-removed budget check code.

For any remaining `OrgID` references in SQL queries: remove the column from SELECT/WHERE clauses.

- [ ] **Step 3: Iterate until clean build**

```bash
go build ./...
```

Expected: clean build.

- [ ] **Step 4: Run all tests**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "fix: resolve remaining org references across Go codebase"
```

---

### Task 9: Clean up models — remove deprecated OrgID fields

Now that NO consumer references `OrgID`, it's safe to remove from the structs.

**Files:**
- Modify: `internal/models/models.go`
- Modify: `internal/models/task.go`
- Modify: `internal/models/agent.go`
- Modify: `internal/ctxutil/ctxutil.go`

- [ ] **Step 1: Remove OrgID from Task struct**

In `internal/models/task.go`, delete the `OrgID` field:

```go
// Remove this line:
	OrgID       string          `json:"org_id"`
```

- [ ] **Step 2: Remove OrgID from Agent struct**

In `internal/models/agent.go`, delete the `OrgID` field:

```go
// Remove this line:
	OrgID         string          `json:"org_id"`
```

- [ ] **Step 3: Remove org types from models.go**

In `internal/models/models.go`, delete `Organization`, `OrgListItem`, `OrgMember`, `OrgMemberWithUser` structs and the `"encoding/json"` import if no longer needed.

- [ ] **Step 4: Remove old org context functions from ctxutil**

In `internal/ctxutil/ctxutil.go`, delete `OrgFromCtx`, `SetOrg`, `OrgRoleFromCtx`, `SetOrgRole` and the `ctxKeyOrg`, `ctxKeyOrgRole` constants.

- [ ] **Step 5: Delete RBAC middleware file and replace with simplified version**

Replace `internal/rbac/middleware.go` entirely:

```go
package rbac

import (
	"net/http"

	"taskhub/internal/ctxutil"
	"taskhub/internal/httputil"
)

func RequireRole(minRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := ctxutil.RoleFromCtx(r.Context())
			if !HasRole(role, minRole) {
				httputil.JSONError(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 6: Verify build**

```bash
go build ./...
```

Expected: PASS.

- [ ] **Step 7: Run tests**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "refactor(models): remove deprecated OrgID fields and org context helpers"
```

---

### Task 10: Update frontend — types, API client, SSE

**Files:**
- Modify: `web/lib/types.ts`
- Modify: `web/lib/api.ts`
- Modify: `web/lib/sse.ts`

- [ ] **Step 1: Update types.ts**

Remove: `Organization`, `OrgListItem`, `OrgMember`, `OrgMemberWithUser` interfaces.

Remove `org_id` from `Agent` and `Task` interfaces.

Add to `Agent`:
```typescript
  is_online: boolean;
  last_health_check?: string;
```

Add to `Task`:
```typescript
  source: string;
  caller_task_id?: string;
  template_id?: string;
  template_version?: number;
  policy_applied?: string[];
```

Add to `SubTask`:
```typescript
  matched_skills?: string[];
  attempt_history?: Record<string, unknown>[];
```

Add `"approval_required"` to SubTask status union.

Add new interfaces for `WorkflowTemplate` and `Policy` (for future use).

- [ ] **Step 2: Update api.ts**

Remove `orgs` and `members` sections entirely.

Remove `orgId` parameter from all functions. Change paths from `/api/orgs/${orgId}/...` to `/api/...`.

Remove unused type imports (`Organization`, `OrgListItem`, `OrgMemberWithUser`).

- [ ] **Step 3: Update sse.ts**

Remove `orgId` parameter from `connectSSE`. Change URL from `/api/orgs/${orgId}/tasks/${taskId}/events` to `/api/tasks/${taskId}/events`.

- [ ] **Step 4: Verify TypeScript compilation**

```bash
cd web && npx tsc --noEmit 2>&1 | head -30
```

Expected: will likely show errors in store.ts and pages that still pass orgId. We fix those next.

- [ ] **Step 5: Commit**

```bash
git add web/lib/types.ts web/lib/api.ts web/lib/sse.ts
git commit -m "refactor(web): remove org from types, API client, and SSE"
```

---

### Task 11: Update frontend stores — remove orgId

**Files:**
- Modify: `web/lib/store.ts`

- [ ] **Step 1: Remove OrgStore entirely**

Delete the entire `useOrgStore` definition.

- [ ] **Step 2: Remove orgId from AgentStore**

Change all function signatures: `(orgId: string, ...)` → `(...)`.
Update all `api.agents.*` calls to remove the `orgId` argument.

- [ ] **Step 3: Remove orgId from TaskStore**

Change all function signatures: `(orgId: string, ...)` → `(...)`.
Update all `api.tasks.*`, `api.messages.*` calls.
Update `connectSSE(orgId, taskId, ...)` → `connectSSE(taskId, ...)`.

- [ ] **Step 4: Verify store compiles**

```bash
cd web && npx tsc --noEmit 2>&1 | head -30
```

Expected: errors in pages that call store methods with orgId. Fix next.

- [ ] **Step 5: Commit**

```bash
git add web/lib/store.ts
git commit -m "refactor(web): remove org from Zustand stores"
```

---

### Task 12: Update frontend pages — remove orgId references

**Files:**
- Modify: `web/app/page.tsx`
- Modify: `web/app/tasks/[id]/page.tsx`
- Modify: `web/app/agents/page.tsx`
- Modify: `web/app/agents/[id]/page.tsx`
- Modify: `web/app/agents/register/page.tsx`
- Modify: any components in `web/components/` that reference orgId

- [ ] **Step 1: For each page file, apply the pattern**

For every `.tsx` file:
1. Remove `useOrgStore` imports
2. Remove `const { currentOrg } = useOrgStore()` or similar
3. Remove `orgId` / `currentOrg.id` from all store/api calls
4. Remove any org selection UI or `if (!currentOrg)` guards

- [ ] **Step 2: Verify frontend build**

```bash
cd web && npm run build
```

Expected: clean build.

- [ ] **Step 3: Run frontend type check**

```bash
cd web && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add web/
git commit -m "refactor(web): remove org references from all pages and components"
```

---

### Task 13: Final verification

- [ ] **Step 1: Full backend build**

```bash
cd /Users/jasper/Documents/code/kokorolabs33/taskhub && go build ./...
```

- [ ] **Step 2: Backend tests**

```bash
go test ./...
```

- [ ] **Step 3: Frontend build + typecheck**

```bash
cd web && npm run build && npx tsc --noEmit
```

- [ ] **Step 4: Full quality gate**

```bash
make check
```

Expected: all checks pass.

- [ ] **Step 5: Review git log**

```bash
git log --oneline feat/meta-agent-foundation ^main
```

Verify clean commit history with no broken intermediate states.
