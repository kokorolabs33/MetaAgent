# Coding Conventions

**Analysis Date:** 2026-04-04

## Naming Patterns

### Go Files

**Functions and Methods:**
- Exported: `PascalCase` (e.g., `UserFromCtx`, `RequireAuth`, `HandleJSONRPC`)
- Unexported: `camelCase` (e.g., `mapStatusToA2AState`, `pollUntilTerminal`, `decodeJSON`)
- Test functions: `TestXxx` format (e.g., `TestRequireAuth_LocalMode_InjectsLocalUser`)

**Variables:**
- Short variable names preferred: `ctx` for context, `t` for *testing.T, `w` for http.ResponseWriter, `r` for *http.Request, `rec` for httptest.Recorder
- Loop variables: `i`, `j`, `err`, `ok`, etc.
- Struct fields: `PascalCase` for exported, `camelCase` for unexported

**Constants:**
- Exported: `PascalCase` (if needed)
- Unexported: `camelCase` (e.g., `ctxKeyUser`, `ctxKeyRole` in `internal/ctxutil/ctxutil.go:9-13`)

**Packages:**
- Single word, lowercase: `ctxutil`, `models`, `handlers`, `executor`, `audit`
- Import paths use `taskhub` prefix (local prefixes configured in `.golangci.yml`)

**JSON Tags:**
- Use `snake_case` for JSON field names to match frontend interfaces
- Examples: `json:"id"`, `json:"avatar_url"`, `json:"created_at"`, `json:"auth_provider_id"`
- Omit field with `json:"field,omitempty"` for optional fields
- See `internal/models/models.go:7-26` for struct tag examples

### TypeScript Files

**Components:**
- File names: `PascalCase` (e.g., `EmptyState.tsx`, `TaskBar.tsx`)
- Exported component names: `PascalCase`
- React hooks with names starting with `use`: `useCallback`, `useRef`, `useState`

**Utilities and Libraries:**
- File names: `camelCase` (e.g., `api.ts`, `types.ts`, `store.ts`, `sse.ts`)
- Helper functions: `camelCase`

**Interfaces and Types:**
- Interfaces: `PascalCase` (e.g., `User`, `Agent`, `Task`, `Message`)
- Type aliases: `PascalCase` (used for unions/intersections)
- Example interfaces in `web/lib/types.ts:1-149`

**Variables:**
- Constants: `camelCase` or `UPPERCASE` based on context (e.g., `suggestions`, `BASE` in `web/lib/api.ts`)
- State variables: `camelCase` (e.g., `isLoading`, `isCreating`, `currentTask`)
- Event handlers: `handleXxx` pattern (e.g., `handleSubmit`, `handleKeyDown`)

## Code Style

### Go Style

**Formatting:**
- `gofmt` enforced (checked in pre-commit and CI)
- `goimports` enabled for import organization (`.golangci.yml:50-52`)
- No manual grouping of imports — tool handles it with local prefix `taskhub`

**Error Handling:**
- Always check errors, never silently ignore with `_ =`
- Exception: `godotenv.Load()` is excluded in linter config (`.golangci.yml:32-33`)
- Use early returns for error cases (standard Go idiom)
- Example pattern in `cmd/server/main.go:36-44`:
```go
pool, err := db.Open(ctx, cfg.DatabaseURL)
if err != nil {
  log.Fatalf("db open: %v", err)
}
```

**Control Flow:**
- Early returns preferred over nested if/else
- Use `defer` for cleanup (e.g., `defer pool.Close()`)

**Panic Usage:**
- Never use `panic()` in production code
- Only `log.Fatalf()` in `main()` functions

**Struct Design:**
- Keep handlers thin — business logic in dedicated packages
- Use dependency injection: pass dependencies to handler constructors
- Example in `cmd/server/main.go:62-80`: handlers receive DB, resolver, broker, etc.

### TypeScript Style

**Strict Mode:**
- TypeScript strict mode enabled in `web/tsconfig.json:7`
- Never add `@ts-ignore` or `@ts-nocheck`
- ESLint enforces `@typescript-eslint/no-explicit-any: "error"` in `web/eslint.config.mjs:11`

**Types:**
- Use `interface` for data shapes (e.g., `User`, `Agent`, `Task` in `web/lib/types.ts`)
- Use `type` for unions/intersections
- Avoid `any` — use `unknown` with type guards if needed
- Example from `web/lib/api.ts:23-26`:
```typescript
async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`, { credentials: "include" });
  if (!res.ok) throw new Error(`GET ${path} -> ${res.status}`);
  return res.json() as Promise<T>;
}
```

**React Patterns:**
- "use client" directive for client components
- `useCallback` for event handlers to prevent unnecessary re-renders
- `useRef` for uncontrolled DOM access
- Zustand for global state (no prop drilling) — see `web/lib/store.ts`

**State Management:**
- Only Zustand stores allowed (not Context, Redux, etc.)
- Zustand stores defined with `create<StoreName>((set, get) => ({ ... }))`
- Example in `web/lib/store.ts:19-44`: agent store with `loadAgents`, `registerAgent`, `deleteAgent`

**Error Messages:**
- Clear, action-oriented messages
- Include endpoint info: `GET /path -> 404`
- Example in `web/lib/api.ts:25`: `throw new Error(\`GET ${path} -> ${res.status}\`)`

**Styling:**
- Tailwind CSS utility classes only
- shadcn/ui components from `web/components/ui/`
- Use Tailwind's responsive classes (e.g., `flex h-full flex-col items-center justify-center px-4`)
- Example in `web/components/conversation/EmptyState.tsx:56-72`

## Import Organization

### Go

**Order (enforced by goimports):**
1. Standard library: `fmt`, `log`, `context`, `net/http`
2. Third-party: `github.com/`, `gorm`, etc.
3. Local (taskhub): `taskhub/internal/...`

**Local Import Prefix:**
- Configured in `.golangci.yml:51-52`: `taskhub`
- All internal imports use this prefix: `import "taskhub/internal/models"`

### TypeScript

**Order:**
1. Type imports: `import type { ... } from ...`
2. Component/library imports: `import { ... } from ...`
3. Relative imports with `@/` alias (configured in `web/tsconfig.json:21-22`)

**Path Aliases:**
- `@/*` maps to web root (e.g., `@/components/ui/button`)
- Use aliases to avoid relative paths (`../../../`)

## Error Handling

**Go Patterns:**

Error checking is mandatory. Standard pattern:
```go
result, err := someFunc()
if err != nil {
  return fmt.Errorf("context: %w", err) // or log.Fatalf if in main
}
```

Check field: `internal/models/models_test.go:38-42` shows how audit tests verify error handling in JSON serialization.

**TypeScript Patterns:**

Errors from API calls are thrown as strings with status codes:
```typescript
if (!res.ok) throw new Error(`POST ${path} -> ${res.status}`);
```

Calling code catches with try/catch:
```typescript
try {
  const agents = await api.agents.list(q);
  set({ agents, isLoading: false });
} catch {
  set({ isLoading: false });
}
```

## Logging

### Go

**Framework:** Standard `log` package

**Usage:**
- `log.Printf()` for informational messages (e.g., `cmd/server/main.go:33`: `log.Printf("TaskHub — mode: %s", cfg.Mode)`)
- `log.Fatalf()` only in `main()` for fatal errors
- Example: `cmd/server/main.go:38`: `log.Fatalf("db open: %v", err)`

**No structured logging library** — keep it simple with Printf/Fatalf

### TypeScript

**Framework:** `console` (no external logger)

**Rules:**
- `console.warn()` and `console.error()` allowed
- `console.log()` and `console.debug()` flagged as warnings by ESLint (`web/eslint.config.mjs:18`)
- Use `console.error()` for errors in state management catch blocks

## Comments

**When to Comment (Go):**
- Public functions/types: must have godoc comment starting with name
- Complex business logic: explain the why, not the what
- Non-obvious decisions: trade-offs, performance notes

**Example from `internal/ctxutil/ctxutil.go`:**
```go
// Package-level comment not shown, but functions have no comments (self-documenting names)
```

**JSDoc/TSDoc:**
- Not consistently used in codebase
- Functions are self-documenting via type signatures
- If adding JSDoc, use standard format:
```typescript
/**
 * Sends a message to the task's conversation.
 * @param taskId - The task ID
 * @param content - Message content
 */
function sendMessage(taskId: string, content: string): Promise<Message> { ... }
```

## Function Design

### Go Functions

**Size Guidelines:**
- Keep functions small and focused
- Test files show typical function patterns: `internal/handlers/handlers_test.go:11-31` shows a test for a small, focused decode function

**Parameters:**
- First parameter: `context.Context` if the function does I/O
- Use named return values when returning multiple values: Example in `internal/models/models.go:28-33`
- Receiver (method): `(receiver *ReceiverType)` not `(this *ReceiverType)`

**Return Values:**
- Standard Go pattern: `(result T, err error)`
- Named returns optional but clear
- Example: `func NewPageRequest(cursor string, limit int) PageRequest` (line 28 in models.go)

### TypeScript Functions

**Async Functions:**
- Use `async/await`, not `.then()` chains
- Example in `web/lib/api.ts:23-26`: all API functions are async
- Return typed Promises: `Promise<T>`

**Callbacks:**
- Wrap event handlers in `useCallback()` to prevent unnecessary re-renders
- Example in `web/components/conversation/EmptyState.tsx:24-43`:
```typescript
const handleSubmit = useCallback(
  async (text?: string) => { ... },
  [input, isCreating, createConversation, router],
);
```

**Parameters:**
- Use object destructuring for multiple parameters
- Example in `web/lib/api.ts:82-89`: `list(params?: { status?: string; q?: string; page?: number; per_page?: number })`

## Module Design

### Go Modules

**Exports:**
- Exported types/functions: `PascalCase`
- Unexported: `camelCase`
- Example from `internal/ctxutil/ctxutil.go`: `UserFromCtx` (exported), `ctxKeyUser` (unexported constant)

**Barrel Files:**
- Not used — each package is small and focused
- Imports are direct: `import "taskhub/internal/models"`

**Dependency Injection:**
- Handlers and services receive dependencies as struct fields
- Example in `cmd/server/main.go:62-80`: `AgentHandler{DB, Resolver}`, `DAGExecutor{DB, Broker, ...}`

### TypeScript Modules

**Exports:**
- Named exports for utils, stores, types: `export const useAgentStore = ...`
- Default export for page components (Next.js requirement)

**Barrel Files:**
- API client is a barrel: `web/lib/api.ts` exports single `api` object with namespaced methods
- Example from `web/lib/api.ts:59-150`:
```typescript
export const api = {
  auth: { login, me, logout },
  agents: { list, get, create, update, delete, ... },
  tasks: { list, get, create, ... },
  // ...
}
```

**Zustand Stores:**
- Each store is its own file or grouped logically
- Example: `web/lib/store.ts` contains `useAgentStore`, `useTaskStore`
- Stores use closure pattern: `create<StoreName>((set, get) => ({ ... }))`

## Frontend-Backend Type Synchronization

**Contract:**
- Go models in `internal/models/models.go` are source of truth
- TypeScript interfaces in `web/lib/types.ts` MUST match Go JSON tags exactly
- Example synchronization:
  - Go struct: `type User struct { ID string \`json:"id"\` ... }`
  - TS interface: `export interface User { id: string; ... }`

**When Modifying Models:**
1. Update Go struct with proper `json:"field_name"` tag
2. Add SQL migration if it's a new column
3. Update TypeScript interface to match
4. Update API client functions if endpoint signature changes
   - Example: `web/lib/api.ts:92`: `tasks.get(id: string)` returns `TaskWithSubtasks`

---

*Conventions analysis: 2026-04-04*
