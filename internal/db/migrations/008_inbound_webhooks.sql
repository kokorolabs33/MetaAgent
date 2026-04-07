CREATE TABLE IF NOT EXISTS inbound_webhooks (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    provider        TEXT NOT NULL DEFAULT 'generic',  -- github, slack, generic
    secret          TEXT NOT NULL DEFAULT '',
    previous_secret TEXT NOT NULL DEFAULT '',           -- dual-secret rotation (D-03)
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_by      TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    delivery_id TEXT PRIMARY KEY,
    webhook_id  TEXT NOT NULL REFERENCES inbound_webhooks(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_created_at ON webhook_deliveries(created_at);
