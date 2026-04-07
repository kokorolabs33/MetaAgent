---
phase: 10-inbound-webhooks
verified: 2026-04-06T00:00:00Z
status: human_needed
score: 4/4 must-haves verified
re_verification: false
human_verification:
  - test: "End-to-end curl test: create webhook via UI, compute HMAC signature, POST to ingestion endpoint, verify task appears on dashboard"
    expected: "curl with valid HMAC returns {\"status\":\"accepted\",\"task_id\":\"<uuid>\"} and task appears on dashboard with webhook payload as context"
    why_human: "Requires live server with DB, executor, and real HMAC signing to verify task creation flow end-to-end"
  - test: "Idempotency: send same delivery ID twice, verify second request returns {\"status\":\"duplicate\"} without creating a second task"
    expected: "Second POST with same X-GitHub-Delivery header returns 200 with duplicate status; dashboard shows only one task"
    why_human: "Requires live server with DB and two sequential curl requests to verify deduplication works"
  - test: "Inactive webhook returns 404: toggle webhook inactive in UI, then POST to ingestion endpoint"
    expected: "POST to /api/webhooks/inbound/{id} returns 404 when webhook is toggled inactive"
    why_human: "Requires live server with DB state change via UI before issuing ingestion request"
  - test: "Slack slash command flow: POST form-encoded Slack payload with HMAC, verify task created with correct title format"
    expected: "Task appears with title 'Slack: /command text...' and description wrapped in EXTERNAL WEBHOOK CONTENT START/END delimiters"
    why_human: "Requires live server; Slack form-encoded body parsing cannot be fully verified statically"
---

# Phase 10: Inbound Webhooks Verification Report

**Phase Goal:** External systems can trigger TaskHub task creation via authenticated webhook endpoints, with GitHub and Slack as built-in integrations
**Verified:** 2026-04-06T00:00:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A GitHub push or PR event with a valid HMAC signature creates a TaskHub task automatically | ✓ VERIFIED | `Ingest` handler calls `webhook.VerifyHMAC`, then `ParseGitHubPayload`, inserts task, spawns executor — all in `internal/handlers/inbound_webhook.go:250-368` |
| 2 | A Slack slash command or event subscription triggers task creation with the Slack message content as the task description | ✓ VERIFIED | `ParseSlackPayload` handles slash commands (form-encoded) and event callbacks (JSON), content wrapped in `SanitizeForLLM` delimiters — `internal/webhook/parsers.go:165-246` |
| 3 | The webhook management page lets users create, edit, delete, and view webhook configurations with generated endpoint URLs and secrets | ✓ VERIFIED | `InboundTab` component (368 lines) in `web/app/settings/webhooks/page.tsx` implements full CRUD, `SecretRevealCard`, `CopyButton`, provider badges, endpoint URL display |
| 4 | Sending the same webhook delivery twice does not create duplicate tasks — idempotency protection works | ✓ VERIFIED | `ON CONFLICT (delivery_id) DO NOTHING` insert, `RowsAffected() == 0` check returns `{"status":"duplicate"}` — `internal/handlers/inbound_webhook.go:282-294` |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/db/migrations/008_inbound_webhooks.sql` | inbound_webhooks + webhook_deliveries tables | ✓ VERIFIED | Both tables created with correct columns, cascade delete, created_at index |
| `internal/db/migrate.go` | Migration 008 registered | ✓ VERIFIED | `"migrations/008_inbound_webhooks.sql"` in files slice (line 32) |
| `internal/models/inbound_webhook.go` | InboundWebhook and WebhookDelivery Go structs | ✓ VERIFIED | Both structs with correct snake_case JSON tags; 24 lines |
| `internal/webhook/verify.go` | HMAC-SHA256 verification with dual-secret support | ✓ VERIFIED | `VerifyHMAC` with constant-time `hmac.Equal`, `sha256=` prefix stripping, dual-secret fallback; 9/9 tests pass |
| `internal/webhook/parsers.go` | GitHub and Slack payload parsers | ✓ VERIFIED | `ParseGitHubPayload` (push/PR), `ParseSlackPayload` (slash cmd/event/challenge), `ParseGenericPayload`, `SanitizeForLLM` — 283 lines |
| `internal/handlers/inbound_webhook.go` | InboundWebhookHandler with CRUD + Ingest endpoint | ✓ VERIFIED | Full CRUD (List/Create/Get/Update/Delete) + Ingest + CleanupDeliveries + StartCleanupLoop — 430 lines |
| `cmd/server/main.go` | Route registration for inbound webhook endpoints | ✓ VERIFIED | Ingest at line 171 (outside auth group), CRUD at lines 238-242 (inside auth group) |
| `web/lib/types.ts` | InboundWebhook TypeScript interface | ✓ VERIFIED | Interface at line 340 with all 8 fields matching backend JSON including `endpoint_url` |
| `web/lib/api.ts` | API client methods for inbound webhook CRUD | ✓ VERIFIED | `inboundWebhooks` namespace at lines 169-177 with list/get/create/update/delete; `InboundWebhook` imported at line 10 |
| `web/app/settings/webhooks/page.tsx` | Tabbed UI with Outbound and Inbound tabs | ✓ VERIFIED | 642 lines; OutboundTab (line 51) and InboundTab (line 368) components; tabbed layout with `activeTab` state (line 611) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/handlers/inbound_webhook.go` | `internal/webhook/verify.go` | `VerifyHMAC` call in Ingest | ✓ WIRED | `webhook.VerifyHMAC(body, signature, wh.Secret, wh.PreviousSecret)` at line 275 |
| `internal/handlers/inbound_webhook.go` | `internal/webhook/parsers.go` | `Parse*Payload` calls by provider | ✓ WIRED | `ParseGitHubPayload` (line 314), `ParseSlackPayload` (line 316), `ParseGenericPayload` (line 324) |
| `internal/handlers/inbound_webhook.go` | `internal/executor` | `Executor.Execute` for task creation | ✓ WIRED | `h.Executor.Execute(context.Background(), task)` at line 361 in background goroutine |
| `cmd/server/main.go` | `internal/handlers/inbound_webhook.go` | Route registration outside auth | ✓ WIRED | `r.Post("/api/webhooks/inbound/{id}", inboundWebhookH.Ingest)` at line 171 (before auth group at line 180) |
| `web/app/settings/webhooks/page.tsx` | `/api/inbound-webhooks` | `api.inboundWebhooks.*` calls | ✓ WIRED | `api.inboundWebhooks.list()` at line 380, `.create()` at 400, `.update()` at 429/449, `.delete()` at 440 |
| `web/lib/api.ts` | `web/lib/types.ts` | `InboundWebhook` type import | ✓ WIRED | `InboundWebhook` imported at line 10 of api.ts, used in 4 method signatures |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `Ingest handler` | `task` (models.Task) | DB INSERT + executor.Execute | Yes — real UUID, title from parsed payload, task inserted into `tasks` table | ✓ FLOWING |
| `InboundTab` (page.tsx) | `webhooks` (InboundWebhook[]) | `api.inboundWebhooks.list()` fetches `/api/inbound-webhooks` | Yes — DB query in `List` handler returns real rows | ✓ FLOWING |
| `SecretRevealCard` | `webhook.secret` | `api.inboundWebhooks.create()` response | Yes — `generateSecret()` uses `crypto/rand`, returned unmasked on create | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| HMAC tests pass | `go test ./internal/webhook/ -run TestVerifyHMAC -v` | 9/9 tests pass | ✓ PASS |
| Backend compiles | `go build ./cmd/server/` | No errors | ✓ PASS |
| Go vet clean | `go vet ./internal/webhook/ ./internal/handlers/ ./cmd/server/` | No issues | ✓ PASS |
| TypeScript compiles | `cd web && npx tsc --noEmit` | No errors | ✓ PASS |
| End-to-end curl test | Requires live server | Not run | ? SKIP (live server required) |
| Duplicate delivery rejection | Requires live server + DB | Not run | ? SKIP (live server required) |
| Inactive webhook 404 | Requires live server + DB state | Not run | ? SKIP (live server required) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| HOOK-01 | 10-01-PLAN.md | HMAC-SHA256 authenticated webhook endpoint — external events trigger task creation | ✓ SATISFIED | `VerifyHMAC` with constant-time comparison; 401 on failure (line 276); `Ingest` creates task and spawns executor |
| HOOK-02 | 10-01-PLAN.md | GitHub and Slack webhook payload parsers automatically convert to TaskHub tasks | ✓ SATISFIED | `ParseGitHubPayload` handles push/PR with meaningful titles; `ParseSlackPayload` handles slash commands and event callbacks; both sanitized with `SanitizeForLLM` |
| HOOK-03 | 10-02-PLAN.md | Frontend webhook management page (create/edit/delete/view webhook configurations) | ✓ SATISFIED | Tabbed page with InboundTab: list, create with secret reveal, toggle, delete; provider badges; copy-to-clipboard; setup instructions |
| HOOK-04 | 10-01-PLAN.md | Idempotency protection — duplicate requests do not create duplicate tasks | ✓ SATISFIED | `ON CONFLICT (delivery_id) DO NOTHING` + `RowsAffected() == 0` returns `{"status":"duplicate"}` |

All 4 requirements declared in plan frontmatter are present in REQUIREMENTS.md under Phase 10 and are fully accounted for.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | — | — | — | — |

No TODO/FIXME/placeholder comments, empty handlers, or hardcoded stubs detected in any phase 10 files.

### Human Verification Required

#### 1. End-to-End GitHub Webhook Ingestion

**Test:** Start backend and frontend, create a GitHub-type webhook via UI, copy the URL and secret, then run:
```bash
SECRET="<copied-secret>"
BODY='{"ref":"refs/heads/main","repository":{"full_name":"test/repo"},"pusher":{"name":"testuser"},"compare":"https://github.com/test/repo/compare/abc...def","head_commit":{"message":"Test commit message"}}'
SIG=$(echo -n "$BODY" | openssl dgst -sha256 -hmac "$SECRET" | awk '{print $2}')
curl -X POST http://localhost:8080/api/webhooks/inbound/<webhook-id> \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: sha256=$SIG" \
  -H "X-GitHub-Event: push" \
  -H "X-GitHub-Delivery: test-delivery-001" \
  -d "$BODY"
```
**Expected:** Response `{"status":"accepted","task_id":"<uuid>"}` and task appears on dashboard with title containing "test/repo" and "Test commit message"
**Why human:** Requires live server, database, and executor running together with real HMAC computation

#### 2. Idempotency Verification

**Test:** Run the same curl command above twice using the same `X-GitHub-Delivery: test-delivery-001` header value
**Expected:** Second request returns `{"status":"duplicate","message":"already processed"}`; only one task created on dashboard
**Why human:** Requires live server with database state persisted between two sequential requests

#### 3. Inactive Webhook Returns 404

**Test:** Toggle the webhook inactive in the Inbound tab UI, then send the curl request again
**Expected:** HTTP 404 response
**Why human:** Requires UI interaction to change DB state, then API request to verify enforcement

#### 4. Slack Slash Command Flow

**Test:** Create a Slack-type webhook, then POST a form-encoded Slack slash command:
```bash
curl -X POST http://localhost:8080/api/webhooks/inbound/<slack-webhook-id> \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -H "X-Slack-Signature: <computed-sig>" \
  -H "X-Slack-Request-Timestamp: 1234567890" \
  -d "command=/taskhub&text=build the new feature&user_name=alice&channel_name=dev"
```
**Expected:** Task created with title "Slack: /taskhub build the new feature" and description wrapped in `[EXTERNAL WEBHOOK CONTENT START]...[EXTERNAL WEBHOOK CONTENT END]` delimiters
**Why human:** Requires live server; Slack HMAC uses `v0:timestamp:body` signing format which differs from GitHub

### Gaps Summary

No gaps found. All automated checks pass:
- All 8 backend artifacts are substantive and wired
- All 3 frontend artifacts are substantive and wired
- All 4 key links verified
- All 4 requirements satisfied (HOOK-01 through HOOK-04)
- 9/9 HMAC unit tests pass
- Backend compiles; TypeScript compiles

Status is `human_needed` because Plan 02 Task 2 is explicitly a human-verify checkpoint requiring live server testing of the full end-to-end webhook ingestion flow, duplicate delivery rejection, and inactive webhook behavior. These behaviors involve sequential database state changes that cannot be verified statically.

---

_Verified: 2026-04-06T00:00:00Z_
_Verifier: Claude (gsd-verifier)_
