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
