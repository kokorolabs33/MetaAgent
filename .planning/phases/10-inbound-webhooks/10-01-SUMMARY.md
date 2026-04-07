---
phase: 10-inbound-webhooks
plan: 01
subsystem: api
tags: [webhooks, hmac, sha256, github, slack, idempotency, security]

# Dependency graph
requires:
  - phase: 06-demo-readiness
    provides: webhook infrastructure (outbound sender, webhook_configs table)
provides:
  - inbound_webhooks and webhook_deliveries database tables
  - HMAC-SHA256 signature verification with dual-secret rotation
  - GitHub push/PR and Slack slash command/event payload parsers
  - InboundWebhookHandler with CRUD + public ingestion endpoint
  - Content sanitization with LLM-safe delimiters
  - Idempotency protection via delivery deduplication
affects: [10-inbound-webhooks]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "HMAC-SHA256 verification with dual-secret rotation for zero-downtime key rotation"
    - "Content delimiter pattern for LLM prompt injection prevention"
    - "Ack-first webhook ingestion: return 200 immediately, execute task in background"
    - "Provider-specific payload parsers with common ParsedPayload output struct"

key-files:
  created:
    - internal/db/migrations/008_inbound_webhooks.sql
    - internal/models/inbound_webhook.go
    - internal/webhook/verify.go
    - internal/webhook/verify_test.go
    - internal/webhook/parsers.go
    - internal/handlers/inbound_webhook.go
  modified:
    - internal/db/migrate.go
    - cmd/server/main.go

key-decisions:
  - "HMAC verification accepts both sha256= prefixed (GitHub) and raw hex formats for maximum compatibility"
  - "Slack challenge handled both in parser (returns sentinel title) and in Ingest handler (returns challenge directly)"
  - "Delivery IDs for Slack use slack-{timestamp} prefix since Slack has no native delivery ID"

patterns-established:
  - "VerifyHMAC(payload, signature, secret, previousSecret) as reusable HMAC verification primitive"
  - "SanitizeForLLM(content, maxLen) wraps external content in [EXTERNAL WEBHOOK CONTENT START/END] delimiters"
  - "Provider-specific parsers return *ParsedPayload or nil (for ignored events)"

requirements-completed: [HOOK-01, HOOK-02, HOOK-04]

# Metrics
duration: 8min
completed: 2026-04-07
---

# Phase 10 Plan 01: Inbound Webhooks Backend Summary

**HMAC-SHA256 verified webhook ingestion with GitHub/Slack/generic parsers, dual-secret rotation, idempotency deduplication, and LLM content sanitization**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-07T06:30:27Z
- **Completed:** 2026-04-07T06:39:19Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- HMAC-SHA256 signature verification with constant-time comparison, sha256= prefix handling, and dual-secret rotation for zero-downtime key rotation
- GitHub push/PR and Slack slash command/event payload parsers with content sanitization delimiters preventing LLM prompt injection
- Complete inbound webhook lifecycle: CRUD management (behind auth), public ingestion endpoint (HMAC auth), idempotency via delivery deduplication, automatic cleanup
- 9 unit tests covering all HMAC verification scenarios including dual-secret fallback

## Task Commits

Each task was committed atomically:

1. **Task 1: Database schema, models, HMAC verification, and idempotency** - `fd2634b` (feat)
2. **Task 2: InboundWebhookHandler, payload parsers, route registration** - `bdc7109` (feat)

_Note: Task 1 was TDD -- RED+GREEN committed together due to pre-commit hook requiring compilable code_

## Files Created/Modified
- `internal/db/migrations/008_inbound_webhooks.sql` - inbound_webhooks + webhook_deliveries tables with cascade delete and created_at index
- `internal/db/migrate.go` - Registered migration 008
- `internal/models/inbound_webhook.go` - InboundWebhook and WebhookDelivery Go structs with JSON tags
- `internal/webhook/verify.go` - VerifyHMAC with constant-time HMAC-SHA256 comparison, sha256= prefix stripping, dual-secret fallback
- `internal/webhook/verify_test.go` - 9 test cases covering correct/wrong/empty secret, dual rotation, prefix formats
- `internal/webhook/parsers.go` - ParseGitHubPayload (push/PR), ParseSlackPayload (slash cmd/event/challenge), ParseGenericPayload, SanitizeForLLM
- `internal/handlers/inbound_webhook.go` - InboundWebhookHandler: List, Create, Get, Update, Delete (auth'd CRUD), Ingest (public), CleanupDeliveries, StartCleanupLoop
- `cmd/server/main.go` - Route registration: ingestion outside auth group, CRUD inside auth group, cleanup goroutine start

## Decisions Made
- HMAC verification accepts both `sha256=<hex>` (GitHub format) and raw hex formats -- maximizes provider compatibility without configuration
- Slack delivery ID uses `slack-{X-Slack-Request-Timestamp}` since Slack does not provide a native delivery UUID
- Slack URL verification challenge is handled in both the parser (sentinel return) and the Ingest handler (direct challenge response) for robustness
- Secret generation uses `crypto/rand` with 32 bytes (64 hex chars) for cryptographic strength
- Create endpoint shows secret once (unmasked); all subsequent List/Get responses mask secrets as "***" per T-10-04

## Deviations from Plan

None - plan executed exactly as written.

## Threat Mitigations Implemented

All threat model mitigations from the plan were implemented:

| Threat | Mitigation | Implementation |
|--------|-----------|----------------|
| T-10-01 Spoofing | HMAC-SHA256 verification | `webhook.VerifyHMAC` with constant-time comparison |
| T-10-02 Tampering | HMAC covers entire body | Signature computed over full request body |
| T-10-03 Repudiation | Delivery audit trail | `webhook_deliveries` table records every delivery |
| T-10-04 Info Disclosure | Secret masking | `maskSecret()` returns "***" in List/Get responses |
| T-10-05 DoS | Body size limit | `http.MaxBytesReader` limits to 1MB |
| T-10-06 Privilege Escalation | Content sanitization | `SanitizeForLLM` wraps in delimiters, strips control chars, enforces length limits |
| T-10-07 Replay | Idempotency | `ON CONFLICT DO NOTHING` on delivery_id primary key |
| T-10-08 Rotation window | Dual-secret | `VerifyHMAC` checks both current and previous secret |

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Backend complete for inbound webhooks; ready for Plan 02 (frontend management UI)
- All CRUD endpoints available for the management UI to consume
- Ingestion endpoint ready for external system configuration (GitHub/Slack webhook URLs)

## Self-Check: PASSED

All 8 files verified present. Both task commits (fd2634b, bdc7109) verified in git log.

---
*Phase: 10-inbound-webhooks*
*Completed: 2026-04-07*
