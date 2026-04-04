---
phase: 01-foundation
plan: 03
subsystem: infra
tags: [docker, docker-compose, postgresql, nextjs-standalone, agent-seeding]

# Dependency graph
requires: []
provides:
  - "docker-compose.yml for one-click local startup (PostgreSQL + platform)"
  - "Next.js standalone output mode for Docker deployment"
  - "4 demo agents seeded on startup (Engineering, Finance, Legal, Marketing)"
  - "Dockerfile with NEXT_PUBLIC_API_URL and HOSTNAME for container binding"
affects: [01-04, 02-agent-status, open-source-readme]

# Tech tracking
tech-stack:
  added: [postgres-16-alpine, docker-compose]
  patterns: [docker-healthcheck-depends-on, idempotent-agent-seeding, standalone-nextjs-docker]

key-files:
  created: [docker-compose.yml]
  modified: [web/next.config.ts, internal/seed/devseed.go, Dockerfile]

key-decisions:
  - "Used TASKHUB_MODE instead of AUTH_MODE (matches actual config.go env var name)"
  - "Agent endpoints use Docker DNS names (e.g., http://engineering-agent:9001) for compose networking"
  - "Demo agents seeded as is_online=false since agent containers are optional"
  - "Commented-out agent services in docker-compose.yml (require separate OPENAI_API_KEY)"

patterns-established:
  - "Docker Compose healthcheck pattern: service_healthy condition for DB readiness"
  - "Agent seeding pattern: idempotent ON CONFLICT (id) DO UPDATE in devseed.go"
  - "Allowlist secret pragma for local-dev credentials in compose files"

requirements-completed: [FOUND-03]

# Metrics
duration: 4min
completed: 2026-04-04
---

# Phase 01 Plan 03: Docker Compose One-Click Startup Summary

**Docker Compose with PostgreSQL + platform bundled, Next.js standalone output, and 4 demo agents seeded on startup for zero-config local development**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-04T22:40:39Z
- **Completed:** 2026-04-04T22:44:45Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Created docker-compose.yml that starts PostgreSQL 16 and the platform with a single `docker compose up` command
- Enabled Next.js standalone output mode so the Dockerfile can produce a self-contained frontend container
- Added 4 demo agents (Engineering, Finance, Legal, Marketing) seeded automatically on startup in local mode
- Updated Dockerfile with NEXT_PUBLIC_API_URL for frontend builds and HOSTNAME=0.0.0.0 for container binding

## Task Commits

Each task was committed atomically:

1. **Task 1: Add standalone output to next.config.ts and seed demo agents** - `c11c450` (feat)
2. **Task 2: Create docker-compose.yml for one-click startup** - `19a929a` (feat)

## Files Created/Modified
- `docker-compose.yml` - One-click startup config with PostgreSQL + platform services and optional agent containers
- `web/next.config.ts` - Added `output: "standalone"` for Docker-compatible Next.js builds
- `internal/seed/devseed.go` - Added SeedAgents() with 4 demo agents and updated LocalSeedAndLog to call it
- `Dockerfile` - Added NEXT_PUBLIC_API_URL build/runtime env vars and HOSTNAME=0.0.0.0

## Decisions Made
- Used `TASKHUB_MODE: local` in docker-compose.yml instead of plan's `AUTH_MODE: local` -- the actual config.go reads TASKHUB_MODE
- Agent endpoints use Docker Compose service DNS names (e.g., `http://engineering-agent:9001`) so uncommenting agent services in compose "just works"
- Seeded agents set `is_online: false` since the OpenAI agent containers are optional (commented out by default)
- Added `pragma: allowlist secret` comments for local-dev-only PostgreSQL credentials in docker-compose.yml

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Used correct env var name TASKHUB_MODE instead of AUTH_MODE**
- **Found during:** Task 2 (docker-compose.yml creation)
- **Issue:** Plan specified `AUTH_MODE: local` but config.go reads `TASKHUB_MODE` (line 29: `getEnv("TASKHUB_MODE", "local")`)
- **Fix:** Used `TASKHUB_MODE: local` in docker-compose.yml
- **Files modified:** docker-compose.yml
- **Verification:** Confirmed config.go checks `TASKHUB_MODE` not `AUTH_MODE`
- **Committed in:** 19a929a (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix)
**Impact on plan:** Essential correctness fix -- using wrong env var would have meant local mode config was silently ignored. No scope creep.

## Issues Encountered
- Pre-commit hook `detect-secrets` flagged local-dev PostgreSQL password and DATABASE_URL as secrets. Resolved by adding `pragma: allowlist secret` inline comments, which is the standard approach for intentional local-dev credentials.
- Pre-commit hooks (eslint, tsc) initially failed because node_modules were not installed in the worktree. Resolved by running `pnpm install`.

## Known Stubs

None -- all functionality is fully wired.

## Threat Flags

None -- no new security surface introduced beyond what the threat model already covers (T-01-01 local postgres password accepted, T-01-02 API key via env var mitigated).

## User Setup Required

None - no external service configuration required. The docker-compose.yml is self-contained for local development. ANTHROPIC_API_KEY is optional (needed only for task execution, not UI browsing).

## Next Phase Readiness
- Docker infrastructure ready for README quickstart instructions (Plan 04)
- Platform can be demoed via `docker compose up` once built
- Agent seeding foundation ready for Phase 2 agent status visualization

---
*Phase: 01-foundation*
*Completed: 2026-04-04*

## Self-Check: PASSED

All created files verified present. All commit hashes verified in git log.
