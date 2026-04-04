# Testing Patterns

**Analysis Date:** 2026-04-04

## Test Framework

### Go

**Runner:**
- Standard `testing` package (built-in)
- No external framework (no testify, no ginkgo)

**Assertion Library:**
- Manual assertions with if/else and `t.Errorf()`
- No assertion library like testify

**Run Commands:**
```bash
go test ./...              # Run all tests
go test -v ./...           # Verbose output
go test -race ./...        # Detect data races
go test -cover ./...       # Show coverage
go test ./... -run TestXxx # Run specific test
```

**CI Integration (`/.github/workflows/ci.yml:62-63`):**
```bash
go test -race -coverprofile=coverage.out ./...
```
- `-race` flag detects race conditions
- Coverage report uploaded as artifact for PRs

### TypeScript

**Runner:**
- Not configured (no jest.config, vitest.config, or test scripts in `web/package.json`)
- Frontend has no automated tests currently
- Type checking via TypeScript compiler (`pnpm tsc --noEmit`)

**Assertion Library:**
- Not in use

**Linting/Type Check Commands:**
```bash
cd web && pnpm lint                # ESLint
cd web && pnpm tsc --noEmit        # Type check (no emit)
cd web && pnpm build               # Next.js build (validates code)
```

## Test File Organization

### Go

**Location:**
- Co-located with source code in same package
- Test file in same directory: `internal/handlers/handlers.go` → `internal/handlers/handlers_test.go`

**Naming:**
- `*_test.go` suffix (standard Go convention)
- 19 test files found across internal packages

**File Count:**
- `internal/handlers/handlers_test.go` — handler and request validation tests
- `internal/auth/middleware_test.go` — authentication middleware tests
- `internal/audit/audit_test.go` — audit logging tests
- `internal/executor/executor_test.go` — DAG executor tests
- `internal/a2a/server_test.go` — A2A server tests
- `internal/models/models_test.go` — model serialization tests
- Plus 13 more test files across packages

## Test Structure

### Go Test Pattern

**Basic Structure:**
```go
package handlers

import (
  "encoding/json"
  "net/http"
  "net/http/httptest"
  "strings"
  "testing"
)

func TestDecodeJSON_ValidBody(t *testing.T) {
  body := `{"title":"Test Task","description":"A test"}`
  r := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(body))
  w := httptest.NewRecorder()

  var req createTaskRequest
  err := decodeJSON(w, r, &req)

  if err != nil {
    t.Fatalf("decodeJSON: %v", err)
  }
  if req.Title != "Test Task" {
    t.Errorf("Title = %q, want %q", req.Title, "Test Task")
  }
}
```

**From `internal/handlers/handlers_test.go:11-31`:**
- Uses `httptest.NewRequest()` to create test request
- Uses `httptest.NewRecorder()` to capture response
- Manual assertions with `t.Errorf()` and `t.Fatalf()`
- Clear test names describing what is tested and expected outcome

**Setup Patterns:**
- No setup/teardown functions (`testing.T.Setup` not used)
- Each test creates its own test doubles (mock servers, recorders)
- Example in `internal/executor/executor_test.go:61-80`:
```go
func TestPollUntilTerminal_WaitForCompletion(t *testing.T) {
  var callCount atomic.Int32

  srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    count := callCount.Add(1)
    state := "working"
    if count >= 2 {
      state = "completed"
    }
    // respond with state
  }))
  defer srv.Close()

  // test using srv.URL
}
```

**Assertion Pattern:**
- `t.Errorf(fmt, args...)` for assertion failures (continues test)
- `t.Fatalf(fmt, args...)` for setup failures (stops test)
- Message format: `fieldName = actual, want expected`
- Example from `internal/auth/middleware_test.go:30-35`:
```go
if gotUserID != "local-user" {
  t.Errorf("user ID = %q, want 'local-user'", gotUserID)
}
```

### Test Names

**Convention:**
- Format: `TestFunctionName_Scenario` or `TestFunctionName_Scenario_Expected`
- Examples:
  - `TestDecodeJSON_ValidBody` (line 11 in handlers_test.go)
  - `TestRequireAuth_LocalMode_InjectsLocalUser` (line 12 in auth_test.go)
  - `TestPollUntilTerminal_WaitForCompletion` (line 61 in executor_test.go)
  - `TestHandleJSONRPC_InvalidJSON` (line 36 in a2a/server_test.go)

**Sub-tests:**
- `t.Run()` used for parameterized/table-driven tests
- Example from `internal/a2a/server_test.go:11-34`:
```go
func TestMapStatusToA2AState(t *testing.T) {
  tests := []struct {
    internal string
    want     string
  }{
    {"pending", "submitted"},
    {"planning", "submitted"},
    {"running", "working"},
    // ...
  }

  for _, tt := range tests {
    t.Run(tt.internal, func(t *testing.T) {
      got := mapStatusToA2AState(tt.internal)
      if got != tt.want {
        t.Errorf("mapStatusToA2AState(%q) = %q, want %q", tt.internal, got, tt.want)
      }
    })
  }
}
```

## Mocking

### Go Mocking

**Framework:** No mocking library (no testify, no gomock)

**Patterns:**

1. **HTTP Testing:** Use `httptest` package
   - `httptest.NewRequest()` — create fake requests
   - `httptest.NewRecorder()` — capture responses
   - `httptest.NewServer()` — mock HTTP server
   - Example from `internal/executor/executor_test.go:64-80`:
```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  // mock behavior
  json.NewEncoder(w).Encode(resp)
}))
defer srv.Close()
// test code uses srv.URL
```

2. **Request Body Mocking:** Use `strings.NewReader()`
   - Example from `internal/handlers/handlers_test.go:12-14`:
```go
body := `{"title":"Test Task","description":"A test","template_id":"tmpl-1"}`
r := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(body))
```

3. **Dependency Injection:** Pass mock objects to handlers
   - Example from `internal/auth/middleware_test.go:12-23`:
```go
m := &Middleware{LocalMode: true}

handler := m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  u := ctxutil.UserFromCtx(r.Context())
  // test assertions inside handler
}))

req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
rec := httptest.NewRecorder()
handler.ServeHTTP(rec, req)
```

**What to Mock:**
- HTTP requests/responses (use httptest)
- External services (create test servers with httptest.NewServer)
- Database queries (use dependency injection, pass test implementations)

**What NOT to Mock:**
- Standard library functions
- Helper functions in same package (test through public API)
- Business logic (test actual implementations)

### TypeScript Mocking

**Not configured** — no Jest/Vitest mocking framework in place.

## Fixtures and Factories

### Go

**Test Data:**
- Inline struct literals in tests
- Example from `internal/audit/audit_test.go:8-26`:
```go
e := Entry{
  TaskID:       "task-1",
  SubtaskID:    "sub-1",
  AgentID:      "agent-1",
  ActorType:    "agent",
  ActorID:      "actor-1",
  Action:       "llm_call",
  ResourceType: "subtask",
  ResourceID:   "sub-1",
  Details:      map[string]string{"prompt": "hello"},
  Model:        "claude-3-opus",
  InputTokens:  100,
  OutputTokens: 200,
  CostUSD:      0.015,
  Endpoint:     "https://api.anthropic.com/v1/messages",
  LatencyMs:    1500,
  StatusCode:   200,
}
```

**Location:**
- Inline in test files (no shared fixtures directory)
- Each test creates what it needs

**Builder/Factory Pattern:**
- Used for complex objects
- Example from `internal/models/models.go:28-33`:
```go
func NewPageRequest(cursor string, limit int) PageRequest {
  if limit <= 0 || limit > 100 {
    limit = 20
  }
  return PageRequest{Cursor: cursor, Limit: limit}
}
```

### TypeScript

**No fixtures currently in use** — no test files in web directory.

## Coverage

### Go

**Requirements:**
- No enforced minimum coverage target
- Coverage report generated in CI: `.github/workflows/ci.yml:63`
- Uploaded as artifact for PRs to review

**View Coverage:**
```bash
go test ./... -cover                      # Summary
go test ./... -coverprofile=coverage.out  # Detailed
go tool cover -html=coverage.out          # HTML report
```

**Current Status:**
- 19 test files across internal packages
- 201 test functions found (from grep count)
- No specific coverage metrics documented

### TypeScript

**No coverage tooling configured** — no tests in place.

## Test Types

### Go Unit Tests

**Scope:**
- Individual functions/methods
- Input validation
- Business logic
- Example: `internal/handlers/handlers_test.go` tests `decodeJSON()` with valid/invalid/empty bodies

**Approach:**
- Isolate function under test
- Mock external dependencies
- Assert return values and side effects

**Examples:**
- `TestDecodeJSON_ValidBody` — validates JSON parsing
- `TestRequireAuth_LocalMode_InjectsLocalUser` — validates auth middleware behavior
- `TestEntry_JSONSerialization` — validates model marshaling/unmarshaling

### Go Integration Tests

**Limited integration tests** — mostly unit-level tests

**Examples with HTTP Integration:**
- `internal/executor/executor_test.go:61-80` — tests polling with mock HTTP server
- `internal/a2a/server_test.go` — tests JSON-RPC request handling with httptest

**Approach:**
- Use `httptest.NewServer()` to mock external HTTP services
- Test handler behavior with full request/response cycle

### E2E Tests

**Not implemented** — no end-to-end tests in codebase

## Common Patterns

### Go Async Testing

**Pattern with Channels:**
```go
// Not shown in detail, but used in executor_test.go for async polling tests
// Typically: use goroutines with channels for async behavior
// or use atomic counters (see line 62 in executor_test.go: var callCount atomic.Int32)
```

**Example from `internal/executor/executor_test.go:61-80`:**
```go
var callCount atomic.Int32

srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  count := callCount.Add(1)
  state := "working"
  if count >= 2 {
    state = "completed"
  }
  // respond based on call count
}))
```

### Go Error Testing

**Pattern:**
- Call function expecting error
- Check `if err == nil` or `if err != nil`
- Verify error message/type

**Example from `internal/handlers/handlers_test.go:33-42`:**
```go
func TestDecodeJSON_InvalidBody(t *testing.T) {
  r := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader("not json"))
  w := httptest.NewRecorder()

  var req createTaskRequest
  err := decodeJSON(w, r, &req)

  if err == nil {
    t.Fatal("expected error for invalid JSON")
  }
}
```

**Another Example from `internal/auth/middleware_test.go:67-94`:**
```go
func TestRequireAuth_CloudMode_NoCookie_Returns401(t *testing.T) {
  m := &Middleware{LocalMode: false, Sessions: nil}

  handler := m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    t.Error("handler should not be called without cookie")
  }))

  req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
  rec := httptest.NewRecorder()
  handler.ServeHTTP(rec, req)

  if rec.Code != http.StatusUnauthorized {
    t.Errorf("status = %d, want 401", rec.Code)
  }

  var body map[string]string
  if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
    t.Fatalf("decode response: %v", err)
  }
  if body["error"] != "unauthorized" {
    t.Errorf("error = %q, want 'unauthorized'", body["error"])
  }
}
```

### Go Context Testing

**Pattern:**
- Create context with test values
- Call function with context
- Verify context was used correctly

**Example from `internal/auth/middleware_test.go:12-39`:**
```go
func TestRequireAuth_LocalMode_InjectsLocalUser(t *testing.T) {
  m := &Middleware{LocalMode: true}

  var gotUserID, gotRole string
  handler := m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    u := ctxutil.UserFromCtx(r.Context())
    if u != nil {
      gotUserID = u.ID
    }
    gotRole = ctxutil.RoleFromCtx(r.Context())
    w.WriteHeader(http.StatusOK)
  }))

  req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
  rec := httptest.NewRecorder()

  handler.ServeHTTP(rec, req)

  if gotUserID != "local-user" {
    t.Errorf("user ID = %q, want 'local-user'", gotUserID)
  }
}
```

### Go JSON Marshaling Tests

**Pattern:**
- Create struct with test values
- Marshal to JSON
- Unmarshal back
- Verify round-trip integrity

**Example from `internal/audit/audit_test.go:8-83`:**
```go
func TestEntry_JSONSerialization(t *testing.T) {
  e := Entry{
    TaskID:       "task-1",
    SubtaskID:    "sub-1",
    AgentID:      "agent-1",
    // ... populate all fields
  }

  data, err := json.Marshal(e)
  if err != nil {
    t.Fatalf("marshal: %v", err)
  }

  var got Entry
  if err := json.Unmarshal(data, &got); err != nil {
    t.Fatalf("unmarshal: %v", err)
  }

  if got.TaskID != e.TaskID {
    t.Errorf("TaskID = %q, want %q", got.TaskID, e.TaskID)
  }
  // ... verify all fields match
}
```

## Test Coverage Gaps

**Untested Areas:**
- Frontend components (no test framework set up)
- Integration between backend components (limited integration tests)
- Database migrations (no explicit migration tests)
- Webhook event delivery (minimal testing)

**High-Priority Missing Tests:**
- End-to-end task flows (task creation → agent assignment → completion)
- Message streaming (SSE event handling)
- Concurrent task execution

**Current Coverage Strengths:**
- Handler validation (request parsing, response format)
- Authentication middleware (local mode, cloud mode, cookie handling)
- Model serialization (JSON round-trip integrity)
- A2A protocol (JSON-RPC validation)

---

*Testing analysis: 2026-04-04*
