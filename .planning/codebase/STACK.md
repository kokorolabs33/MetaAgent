# Technology Stack

**Analysis Date:** 2026-04-04

## Languages

**Primary:**
- Go 1.26.1 - Backend API server (`cmd/server/main.go`), task orchestration, and execution engine
- TypeScript 5.x - Next.js frontend application (`web/`)

**Secondary:**
- JavaScript (via package.json scripts) - Build tooling and frontend automation

## Runtime

**Environment:**
- Go 1.26.1 - Backend compilation and execution
- Node.js 22.x (Bookworm) - Frontend development and Next.js server
- Docker (multi-stage builds) - Container deployment

**Package Manager:**
- Go modules (`go.mod`, `go.sum`) - Backend dependency management
- pnpm - Frontend package manager (`web/package.json`)
  - Lockfile: `web/pnpm-lock.yaml` (committed)

## Frameworks

**Backend:**
- Chi v5.2.5 - HTTP router and middleware (`github.com/go-chi/chi/v5`)
- Chi CORS v1.2.2 - Cross-origin request handling (`github.com/go-chi/cors`)
- pgx v5.9.1 - PostgreSQL driver with connection pooling (`github.com/jackc/pgx/v5`)
- godotenv v1.5.1 - Environment variable loading (`github.com/joho/godotenv`)
- google/uuid v1.6.0 - UUID generation for IDs (`github.com/google/uuid`)

**Frontend:**
- Next.js 16.1.6 - React meta-framework for SSR and static generation
- React 19.2.3 - UI component library
- React DOM 19.2.3 - DOM rendering
- TypeScript 5.x - Type safety for all frontend code

**UI/Styling:**
- Tailwind CSS v4 - Utility-first CSS framework (`@tailwindcss/postcss`)
- shadcn/ui components - Pre-built accessible UI components
- Lucide React v0.577.0 - Icon library
- Class Variance Authority v0.7.1 - Component variant management
- clsx v2.1.1 - Conditional class name utility
- tailwind-merge v3.5.0 - Tailwind class merging

**Visualization:**
- React Flow (@xyflow/react) v12.10.1 - DAG visualization for task execution plans
- react-markdown v10.1.0 - Markdown rendering for rich content

**State Management:**
- Zustand v5.0.11 - Lightweight client-side state management (`web/lib/store.ts`)

**Miscellaneous:**
- tw-animate-css v1.4.0 - Animation utilities
- Base UI (@base-ui/react) v1.3.0 - Headless UI component library

## Key Dependencies

**Critical:**
- jackc/pgx/v5 - PostgreSQL client with connection pooling; essential for database operations
- chi/v5 - HTTP routing foundation for all API endpoints
- google/uuid - ID generation for tasks, users, agents (used throughout the system)
- React 19.2.3 - Core frontend UI framework
- Next.js 16.1.6 - Server-side rendering and frontend routing

**Infrastructure:**
- chi/cors - CORS configuration for frontend requests from `http://localhost:3000` (dev) or configured `FRONTEND_URL`
- godotenv - Environment variable loading for configuration
- Tailwind CSS - CSS generation and optimization
- ESLint 9.x - Frontend linting (`web/package.json`)

## Configuration

**Backend (Environment Variables):**
Location: `.env` (not committed), `.env.example` (committed reference)

Required:
- `DATABASE_URL` - PostgreSQL connection string (default: `postgres://localhost:5432/taskhub?sslmode=disable`)
- `ANTHROPIC_API_KEY` - Anthropic API key for LLM-based task orchestration
- `OPENAI_API_KEY` - OpenAI API key for team agents (`cmd/openaiagent`)

Optional:
- `PORT` - Server port (default: `8080`)
- `TASKHUB_MODE` - `local` (no auth) or `cloud` (full auth) (default: `local`)
- `GOOGLE_CLIENT_ID` - Google OAuth client ID (not yet implemented)
- `GOOGLE_CLIENT_SECRET` - Google OAuth client secret (not yet implemented)
- `SESSION_SECRET` - Session encryption key (must be changed in production)
- `TASKHUB_SECRET_KEY` - Symmetric key for encrypting agent auth configs at rest
- `FRONTEND_URL` - Frontend origin for CORS (default: `http://localhost:3000`)

**Frontend (Environment Variables):**
Location: `web/.env.local` (not committed), `web/.env.example` (committed reference)

- `NEXT_PUBLIC_API_URL` - Backend API base URL (default: `http://localhost:8080`)

**Build Configuration:**

Backend:
- `Dockerfile` - Multi-stage Docker build: Go binary in stage 1, Next.js in stage 2, runtime stage 3
- `Makefile` - Build and development commands

Frontend:
- `web/tsconfig.json` - TypeScript configuration with strict mode enabled, path aliases (`@/*` → `./`)
- `web/package.json` - Scripts: `dev`, `build`, `start`, `lint`
- Next.js uses `.next/` directory for build output (not committed)

**Linting & Formatting:**

Backend:
- `.golangci.yml` - golangci-lint configuration
- `gofmt` - Go code formatter (via `make fmt`)

Frontend:
- ESLint 9.x with next config - Linting rules
- Prettier (via pnpm) - Code formatter (`make fmt-frontend`)

## Platform Requirements

**Development:**
- Go 1.26.1 (local build)
- Node.js 22.x (local build for frontend)
- PostgreSQL 12+ (local database)
- Make (build automation)
- docker/docker-compose (optional, for containerized development)
- pre-commit hooks (optional, configured in `.pre-commit-config.yaml`)

**Production:**
- Docker runtime with:
  - Node.js 22-bookworm-slim base image (includes Go binary and Next.js)
  - PostgreSQL 12+ database (external or managed)
  - 8080 port exposed (backend API)
  - 3000 port exposed (Next.js frontend, if served from Docker)

**Database:**
- PostgreSQL 12+ with pgx driver
- Connection pooling via pgxpool
- Migrations: SQL files in `internal/db/migrations/` (embedded via `//go:embed`)

---

*Stack analysis: 2026-04-04*
