# Phase 10: Inbound Webhooks - Context

**Gathered:** 2026-04-07
**Status:** Ready for planning

<domain>
## Phase Boundary

External systems trigger task creation via authenticated webhook endpoints. HMAC-SHA256 signature verification. GitHub (push + PR) and Slack payload parsers. Webhook management UI in existing settings page. Idempotency protection against duplicate requests.

</domain>

<decisions>
## Implementation Decisions

### Webhook Authentication & Security
- **D-01:** HMAC-SHA256 signature verification on all inbound webhooks. Each webhook config has its own secret.
- **D-02:** Webhook endpoint lives outside auth middleware (external systems can't authenticate as users). Authentication is purely via HMAC signature.
- **D-03:** Dual-secret rotation scheme — each webhook has `secret` (active) and `previous_secret` (grace period). Allows rotation without downtime.
- **D-04:** Sanitize webhook payloads before passing to LLM — wrap in content delimiters to prevent prompt injection via crafted commit messages or PR descriptions.

### Payload Parsers
- **D-05:** GitHub webhook parser supports `push` and `pull_request` events. Other events return 200 OK but don't create tasks (ack-first pattern).
- **D-06:** Slack webhook parser supports slash commands and event callbacks. Responds with 200 immediately, creates task asynchronously.
- **D-07:** Each parser extracts: title, description, source metadata (repo, author, URL). Maps to `POST /api/tasks` equivalent.
- **D-08:** New database table `inbound_webhooks` with: id, name, provider (github/slack/generic), secret, previous_secret, org_id, is_active, created_at.
- **D-09:** New migration for `inbound_webhooks` table + `webhook_deliveries` table (for idempotency tracking).

### Management UI
- **D-10:** Add "Inbound" tab to existing `/manage/settings/webhooks` page. Two tabs: "Outbound" (existing) and "Inbound" (new).
- **D-11:** Inbound tab shows: webhook list with name, provider, URL (auto-generated), status (active/inactive). CRUD operations.
- **D-12:** On create: generate webhook URL + secret, show once with copy button. Provider-specific setup instructions (GitHub webhook URL + content type).

### Idempotency
- **D-13:** Store delivery ID (GitHub: `X-GitHub-Delivery`, Slack: `X-Slack-Request-Timestamp`) in `webhook_deliveries` table.
- **D-14:** Before creating task, check if delivery ID exists. If yes, return 200 OK without creating duplicate task.
- **D-15:** Cleanup: delete delivery records older than 7 days (prevent table bloat).

### Claude's Discretion
- Exact webhook URL path format (/api/webhooks/inbound/{id})
- Slack challenge verification for URL verification handshake
- Rate limiting on webhook endpoint
- How to display webhook-triggered tasks differently in the UI (if at all)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Webhook Infrastructure
- `internal/webhook/sender.go` — Outbound webhook HMAC signing pattern (reference for inbound)
- `internal/handlers/webhooks.go` — Outbound webhook CRUD handler
- `web/app/manage/settings/webhooks/page.tsx` — Existing webhook management UI

### Task Creation
- `internal/handlers/tasks.go` — TaskHandler.Create (endpoint to mirror for webhook-triggered tasks)
- `internal/models/task.go` — Task model

### Research
- `.planning/research/PITFALLS.md` — Webhook pitfalls (prompt injection, duplicate tasks, retry windows)
- `.planning/research/ARCHITECTURE.md` — Inbound webhook architecture

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/webhook/sender.go` — HMAC signing implementation (flip for verification)
- Outbound webhook CRUD handler — reuse pattern for inbound CRUD
- Existing webhooks settings page — extend with tab for inbound

### Integration Points
- New route: POST /api/webhooks/inbound/{id} (outside auth middleware)
- New handler: InboundWebhookHandler
- New migration: inbound_webhooks + webhook_deliveries tables
- Existing settings page: add Inbound tab

</code_context>

<specifics>
## Specific Ideas

- Demo scenario: push to GitHub repo → TaskHub automatically creates a code review task → agents analyze the diff
- Webhook URL should be easy to copy and paste into GitHub/Slack settings

</specifics>

<deferred>
## Deferred Ideas

- More providers (Jira, Linear, GitLab) — add parser per provider as needed
- Webhook retry dashboard (show delivery history) — nice to have, not v2.0
- Conditional webhooks (only trigger on certain branches/labels) — future

None — discussion stayed within phase scope

</deferred>

---

*Phase: 10-inbound-webhooks*
*Context gathered: 2026-04-07*
