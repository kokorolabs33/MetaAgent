# Add Feature Skill

Use this skill when adding new functionality to TaskHub.

## Pre-flight

Before writing code, answer:

1. **Which layer?** Backend only / Frontend only / Full-stack
2. **Does it need a new API endpoint?**
3. **Does it need new database tables/columns?**
4. **Does it affect the agent system?** (Master Agent vs Team Agent)
5. **Does it need new SSE events?**

## Full-Stack Feature Checklist

For features that span both backend and frontend:

### Backend (do first — frontend depends on API)

- [ ] **Migration** — `internal/db/migrations/NNN_description.sql`
  - Use `IF NOT EXISTS` for idempotency
  - Add filename to list in `internal/db/migrate.go`
- [ ] **Model** — `internal/models/models.go`
  - Add struct with `json:"snake_case"` tags
- [ ] **Handler** — `internal/handlers/`
  - Create handler struct with `DB *sql.DB` field
  - Use `r.Context()` for all DB queries
  - Use `jsonOK()` / `jsonError()` helpers
  - Validate input at handler boundary
- [ ] **Route** — `cmd/server/main.go`
  - Register under `/api` route group
- [ ] **Audit** — if the feature makes LLM calls, log via audit logger

### Frontend (after backend API is ready)

- [ ] **Types** — `web/lib/types.ts`
  - Mirror Go struct JSON tags exactly
- [ ] **API client** — `web/lib/api.ts`
  - Add typed fetch function
- [ ] **Store** — `web/lib/store.ts`
  - Add state + actions to Zustand store
- [ ] **Component** — `web/components/`
  - PascalCase filename
  - Use shadcn/ui components from `components/ui/`
  - Tailwind for styling

### If adding SSE events

- [ ] Backend: publish via `broker.Publish(channelID, eventType, data)`
- [ ] Types: add to `SSEEventType` union in `web/lib/types.ts`
- [ ] Store: handle in `handleSSEEvent()` in `web/lib/store.ts`

### If adding a new Agent

- [ ] Add definition in `internal/seed/seed.go` (upserted on startup)
- [ ] Unique name, color, description, system prompt
- [ ] Consider how Master Agent should assign tasks to it

## Order of Operations

```
1. Migration (schema)
2. Model (Go struct)
3. Handler (business logic + HTTP)
4. Route registration
5. TypeScript types
6. API client function
7. Zustand store action
8. UI component
9. Wire into page
```

Always build bottom-up: database → backend → API → frontend.

## Verification

After implementing:

1. `make lint` — 0 issues
2. `make build` — both stacks compile
3. `make dev-backend` + `make dev-frontend` — manual test
4. Verify SSE events flow end-to-end if applicable
