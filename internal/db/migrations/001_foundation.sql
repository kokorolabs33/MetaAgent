-- TaskHub V2 Foundation Schema
-- All IDs are TEXT (UUID strings generated in application code)

-- organizations
CREATE TABLE IF NOT EXISTS organizations (
    id                    TEXT PRIMARY KEY,
    name                  TEXT NOT NULL,
    slug                  TEXT NOT NULL UNIQUE,
    plan                  TEXT NOT NULL DEFAULT 'free',
    settings              JSONB NOT NULL DEFAULT '{}',
    budget_usd_monthly    DECIMAL(10,2) NOT NULL DEFAULT 0,
    budget_alert_threshold DECIMAL(5,2) NOT NULL DEFAULT 0.8,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- users
CREATE TABLE IF NOT EXISTS users (
    id               TEXT PRIMARY KEY,
    email            TEXT NOT NULL UNIQUE,
    name             TEXT NOT NULL DEFAULT '',
    avatar_url       TEXT NOT NULL DEFAULT '',
    auth_provider    TEXT NOT NULL DEFAULT '',
    auth_provider_id TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- org_members: maps users to organizations with a role
CREATE TABLE IF NOT EXISTS org_members (
    org_id    TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role      TEXT NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (org_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_org_members_user_id ON org_members(user_id);

-- auth_sessions
CREATE TABLE IF NOT EXISTS auth_sessions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_user_id ON auth_sessions(user_id);

-- agents
CREATE TABLE IF NOT EXISTS agents (
    id             TEXT PRIMARY KEY,
    org_id         TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    version        TEXT NOT NULL DEFAULT '1.0.0',
    description    TEXT NOT NULL DEFAULT '',
    endpoint       TEXT NOT NULL DEFAULT '',
    adapter_type   TEXT NOT NULL DEFAULT 'native' CHECK (adapter_type IN ('http_poll', 'native')),
    adapter_config JSONB NOT NULL DEFAULT '{}',
    auth_type      TEXT NOT NULL DEFAULT '',
    auth_config    JSONB NOT NULL DEFAULT '{}',
    capabilities   JSONB NOT NULL DEFAULT '[]',
    input_schema   JSONB NOT NULL DEFAULT '{}',
    output_schema  JSONB NOT NULL DEFAULT '{}',
    config         JSONB NOT NULL DEFAULT '{}',
    status         TEXT NOT NULL DEFAULT 'active',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, name)
);

-- agent_user_permissions: per-user permission on an agent
CREATE TABLE IF NOT EXISTS agent_user_permissions (
    org_id     TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    agent_id   TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission TEXT NOT NULL,
    PRIMARY KEY (org_id, agent_id, user_id, permission)
);

-- agent_role_permissions: per-role permission on an agent
CREATE TABLE IF NOT EXISTS agent_role_permissions (
    org_id     TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    agent_id   TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    role       TEXT NOT NULL,
    permission TEXT NOT NULL,
    PRIMARY KEY (org_id, agent_id, role, permission)
);

-- tasks
CREATE TABLE IF NOT EXISTS tasks (
    id           TEXT PRIMARY KEY,
    org_id       TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    title        TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'pending',
    created_by   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    metadata     JSONB NOT NULL DEFAULT '{}',
    plan         JSONB,
    result       JSONB,
    error        TEXT,
    replan_count INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_tasks_org_created ON tasks(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_org_status ON tasks(org_id, status);

-- subtasks
CREATE TABLE IF NOT EXISTS subtasks (
    id            TEXT PRIMARY KEY,
    task_id       TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id      TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    instruction   TEXT NOT NULL DEFAULT '',
    depends_on    TEXT[] NOT NULL DEFAULT '{}',
    status        TEXT NOT NULL DEFAULT 'pending',
    input         JSONB NOT NULL DEFAULT '{}',
    output        JSONB,
    error         TEXT,
    poll_job_id   TEXT,
    poll_endpoint TEXT,
    attempt       INT NOT NULL DEFAULT 0,
    max_attempts  INT NOT NULL DEFAULT 3,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_subtasks_task_status ON subtasks(task_id, status);
CREATE INDEX IF NOT EXISTS idx_subtasks_task_created ON subtasks(task_id, created_at);
CREATE INDEX IF NOT EXISTS idx_subtasks_depends_on ON subtasks USING GIN (depends_on);

-- events
CREATE TABLE IF NOT EXISTS events (
    id         TEXT PRIMARY KEY,
    task_id    TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    subtask_id TEXT REFERENCES subtasks(id) ON DELETE CASCADE,
    type       TEXT NOT NULL,
    actor_type TEXT NOT NULL DEFAULT '',
    actor_id   TEXT NOT NULL DEFAULT '',
    data       JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_events_task_created ON events(task_id, created_at);
CREATE INDEX IF NOT EXISTS idx_events_subtask_created ON events(subtask_id, created_at);

-- messages
CREATE TABLE IF NOT EXISTS messages (
    id          TEXT PRIMARY KEY,
    task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    sender_type TEXT NOT NULL DEFAULT '',
    sender_id   TEXT NOT NULL DEFAULT '',
    sender_name TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL DEFAULT '',
    mentions    TEXT[] NOT NULL DEFAULT '{}',
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_messages_task_created ON messages(task_id, created_at);

-- audit_logs
CREATE TABLE IF NOT EXISTS audit_logs (
    id            TEXT PRIMARY KEY,
    org_id        TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    task_id       TEXT REFERENCES tasks(id) ON DELETE CASCADE,
    subtask_id    TEXT REFERENCES subtasks(id) ON DELETE CASCADE,
    agent_id      TEXT REFERENCES agents(id) ON DELETE CASCADE,
    actor_type    TEXT NOT NULL DEFAULT '',
    actor_id      TEXT NOT NULL DEFAULT '',
    action        TEXT NOT NULL DEFAULT '',
    resource_type TEXT NOT NULL DEFAULT '',
    resource_id   TEXT NOT NULL DEFAULT '',
    details       JSONB NOT NULL DEFAULT '{}',
    model         TEXT NOT NULL DEFAULT '',
    input_tokens  INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    cost_usd      DECIMAL(10,6) NOT NULL DEFAULT 0,
    endpoint_called TEXT NOT NULL DEFAULT '',
    latency_ms    INT NOT NULL DEFAULT 0,
    status_code   INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_org_created ON audit_logs(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_task ON audit_logs(task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_agent ON audit_logs(agent_id, created_at DESC);

-- sessions and session_memory are deferred for now
-- TODO: Add sessions (multi-turn conversation context) and session_memory
-- (long-term agent memory) tables in a future migration.
