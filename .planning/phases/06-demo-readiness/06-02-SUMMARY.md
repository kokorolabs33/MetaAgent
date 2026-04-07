---
phase: "06"
plan: "02"
subsystem: seed-data
tags: [seed, templates, policies, demo-data, database]
dependency_graph:
  requires: []
  provides: [seed-templates, seed-policies, seed-demo, seed-demo-makefile]
  affects: [analytics-dashboard, audit-log, template-list, policy-list]
tech_stack:
  added: []
  patterns: [idempotent-seed, on-conflict-do-nothing, deterministic-ids, transactional-batch-insert]
key_files:
  created:
    - internal/seed/templates.go
    - internal/seed/policies.go
    - internal/seed/demo.go
    - cmd/seeddemo/main.go
  modified:
    - internal/seed/devseed.go
    - Makefile
decisions:
  - "Subtask inserts use WHERE EXISTS on agent name to gracefully skip if agents not registered (FK constraint prevents NULL/empty agent_id)"
  - "Policy rules stored as flat JSON matching PolicyRule struct rather than nested when/require/restrict objects from plan (aligned with actual engine.go types)"
metrics:
  duration: "4min"
  completed: "2026-04-07"
---

# Phase 06 Plan 02: Seed Templates, Policies, and Demo Data Summary

Idempotent seed functions for 6 workflow templates, 5 governance policies, and a demo data script generating 8 tasks with subtasks, events, and template executions for populated manage pages.

## What Was Done

### Task 1: Template and Policy Seed Data Functions

Created `internal/seed/templates.go` with `SeedTemplates` function seeding 6 workflow templates:
- Code Review Pipeline (v3, active) - 3-step security+performance review DAG
- Market Research Report (v2, active) - competitive analysis with executive summary
- Bug Triage Workflow (v1, active) - reproduce and root-cause analysis
- Content Creation Pipeline (v4, active) - research, draft, SEO review
- Security Audit Checklist (v2, active) - OWASP scan + auth review + report
- Onboarding Checklist (v1, inactive) - system access + orientation scheduling

Created `internal/seed/policies.go` with `SeedPolicies` function seeding 5 governance policies:
- Security Review Required (priority 100) - triggers on security keywords
- Budget Approval Threshold (priority 90) - requires approval above 3 subtasks
- Data Access Policy (priority 80) - restricts concurrent subtasks for PII/GDPR
- Compliance Check (priority 70) - requires legal analysis skills
- Rate Limiting (priority 50, inactive) - global subtask limits

Updated `internal/seed/devseed.go` to call both functions from `LocalSeed`, so templates and policies auto-seed on every local-mode server startup.

### Task 2: Demo Seed Script

Created `internal/seed/demo.go` with `SeedDemo` function that inserts within a single transaction:
- 8 tasks (7 completed, 1 failed) spread over 14 days
- 20 subtasks assigned to Engineering/Marketing/Finance/Legal departments via name subquery
- 47 events covering full task lifecycle (planning, planned, running, completed/failed) plus subtask-level events
- 4 template executions linking tasks to seeded templates

Created `cmd/seeddemo/main.go` as standalone binary that loads config, runs migrations, seeds base data, then seeds demo data.

Added `seed-demo` target to Makefile.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Subtask agent_id FK constraint prevents empty string fallback**
- **Found during:** Task 2
- **Issue:** Plan specified using empty string fallback when agent name subquery returns NULL, but `subtasks.agent_id` has `NOT NULL REFERENCES agents(id)` constraint -- empty string would violate FK
- **Fix:** Changed subtask INSERT to use `SELECT ... FROM agents WHERE name = $8` pattern so the row is simply not inserted if the agent doesn't exist (no violation, graceful skip)
- **Files modified:** `internal/seed/demo.go`
- **Commit:** 3faf298

## Commits

| # | Hash | Message |
|---|------|---------|
| 1 | cf4172d | feat(06-02): seed templates and policies on local-mode startup |
| 2 | 3faf298 | feat(06-02): add demo seed script with tasks, events, and template executions |

## Verification

- `go build ./internal/seed/` -- passes
- `go build ./cmd/seeddemo/` -- passes
- `go vet ./internal/seed/ ./cmd/seeddemo/` -- passes
- All pre-commit hooks pass (gofmt, go vet, go build)
- 6 templates with ON CONFLICT DO NOTHING
- 5 policies with ON CONFLICT DO NOTHING
- 8 tasks, 20 subtasks, 47 events, 4 template executions all idempotent
- `seed-demo` Makefile target present

## Self-Check: PASSED

All 4 created files verified on disk. Both commit hashes found in git log.
