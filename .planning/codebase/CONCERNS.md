# Codebase Concerns

**Analysis Date:** 2026-04-04

## Tech Debt

**LLM Integration via Shell Subprocess:**
- Issue: `internal/orchestrator/orchestrator.go:135-146` uses `exec.CommandContext()` to call the `claude` CLI tool directly. This is a temporary MVP solution with known limitations.
- Files: `internal/orchestrator/orchestrator.go:135`
- Impact:
  - Requires `claude` CLI installed and available in $PATH; breaks if missing
  - Subprocess spawning is slower and less reliable than SDK calls
  - Difficult to pass large prompts (shell escaping issues, argument length limits)
  - No built-in retry logic or error recovery
  - Blocks on each LLM call (no streaming, batching, or parallelization)
- Fix approach: Replace `callLLM` function with Anthropic Go SDK (`github.com/anthropics/anthropic-sdk-go`) once available. This will provide native streaming, better error handling, and tool use support.

**Google OAuth Not Implemented:**
- Issue: `internal/auth/handlers.go:29,35` contain TODO comments for incomplete OAuth flow
- Files: `internal/auth/handlers.go`
- Impact:
  - `GoogleLogin` and `GoogleCallback` handlers return 501 Not Implemented
  - Users cannot authenticate via Google SSO; only email-only MVP login works
  - State validation and CSRF protection not implemented
- Fix approach: Implement full OAuth 2.0 flow with state parameter validation, code exchange, and ID token verification. Use a standard OAuth library.

**Hardcoded Magic Numbers Throughout Executor:**
- Issue: Concurrency limits, timeouts, and retry parameters are hardcoded throughout `internal/executor/executor.go`
- Files: `internal/executor/executor.go:54-67` (defaults: 10 global, 3 per-agent), `executor.go:702` (5-minute poll timeout)
- Impact:
  - No ability to tune execution behavior without code changes
  - Global and per-agent limits cannot be adjusted for different workloads
  - Poll timeout is fixed; long-running tasks may timeout prematurely
  - No configuration mechanism exposed
- Fix approach: Move magic numbers to configuration struct passed to DAGExecutor during initialization. Add environment variables or config file support for tuning at deployment time.

**Silenced Error Writes in HTTP Handlers:**
- Issue: Multiple HTTP response encoding errors are silently ignored via `_ =` pattern
- Files:
  - `internal/httputil/httputil.go:11` (JSONError encoding failure ignored)
  - `internal/auth/handlers.go:123,138` (GetMe JSON encoding failures ignored)
  - `internal/handlers/helpers.go:13,19,25` (all JSON response encoding failures ignored)
- Impact:
  - Failed HTTP responses may send partial/corrupt JSON; clients receive malformed data
  - Server logging does not detect serialization failures
  - Makes debugging client-side parsing errors very difficult
- Fix approach: Log all JSON encoding errors to server logs; consider monitoring for serialization failures. At minimum, return 500 status if encoding fails.

**Database Query Errors Silently Ignored in Non-Critical Operations:**
- Issue: Multiple database operations use `_ =` to discard errors that should be logged
- Files:
  - `internal/executor/executor.go:86,125,141,166,194,421,482,540,624,639,791,830,972,977,1107,1131,1155,1179` (multiple locations ignoring DB exec/query errors)
  - `internal/executor/recovery.go:122,137,150,156,162` (recovery routine ignoring DB errors)
  - `internal/a2a/server.go:219` (A2A handler ignoring query row error)
- Impact:
  - Silent failures in task status updates; tasks may get stuck in incorrect states
  - Template version tracking silently fails during execution
  - Recovery routine fails to update subtask states after server restart
  - Difficult to debug why tasks are not progressing
- Fix approach: Log all ignored errors with context (task ID, operation type). Consider adding an operation-level audit log. For recovery, fail fast and log severity level errors.

**Concurrent Goroutine Spawning Without Bounded Pool:**
- Issue: `internal/executor/executor.go:472-475` and `internal/executor/recovery.go:68,129` spawn unbounded goroutines based on number of subtasks and recovery tasks
- Files: `internal/executor/executor.go:329-492`, `internal/executor/recovery.go:68-180`
- Impact:
  - With 1000+ subtasks, could spawn 1000+ goroutines, exceeding system limits
  - Per-agent concurrency limit (max 3) is enforced, but global limit (max 10) uses semaphore channel that may not block correctly under load
  - Recovery routine spawns one goroutine per running task without bounds
  - Memory usage and context switch overhead grows unbounded
- Fix approach: Use worker pool pattern with bounded goroutine count. Implement proper semaphore for global and per-agent limits. Consider workqueue library (e.g., `github.com/cornelk/queue`).

**Context Lifecycle Issues in A2A Task Execution:**
- Issue: `internal/a2a/server.go:132-136` spawns task executor with `context.Background()`, losing cancellation context from parent request
- Files: `internal/a2a/server.go:130-137`
- Impact:
  - A2A-triggered tasks cannot be cancelled via HTTP request context
  - If upstream cancels the A2A HTTP request, the spawned task continues forever
  - Orphaned long-running tasks accumulate, consuming resources
- Fix approach: Store request context at task creation time in database; executor can load and use it. Or pass parent context through task metadata. Implement proper context propagation.

## Known Bugs

**Session Expiration Not Cleaned Up:**
- Symptoms: Expired sessions accumulate in `auth_sessions` table indefinitely
- Files: `internal/auth/session.go:55-57` (DeleteExpired method exists but is never called)
- Trigger: Server running long-term without cleanup
- Workaround: None; database will grow unbounded
- Impact: Database disk usage increases over time; session lookups become slower

**Task Status Inconsistency After Failed Orchestrator Call:**
- Symptoms: Task marked as "failed" but plan field remains null; frontend UI may crash trying to render null plan
- Files: `internal/executor/executor.go:139-145` (planning failure path)
- Trigger: Orchestrator (claude CLI) times out or returns invalid JSON
- Workaround: Manually update task status in database
- Impact: Task appears failed but doesn't clearly show why; users cannot distinguish between orchestrator failures and agent failures

**Unhandled JSON Unmarshal Errors in Aggregator Cache:**
- Symptoms: Agent capability parsing silently falls back to empty array
- Files: `internal/handlers/agents.go:55-57` (unmarshalling error in scanAgent)
- Trigger: Corrupted JSONB data in database
- Workaround: Manually fix database; no detection at runtime
- Impact: Agents lose their capabilities; orchestrator cannot assign work to them

## Security Considerations

**Weak Default Session Secret:**
- Risk: `internal/config/config.go:34` defaults to "change-me-in-production"
- Files: `internal/config/config.go:34`, `internal/auth/handlers.go:119` (7-day MaxAge cookie)
- Current mitigation: Documentation warns to set SESSION_SECRET
- Recommendations:
  - Reject default secret at startup in cloud mode with loud error
  - Generate random default secret per server instance
  - Add configuration validation in `Config.Validate()` method

**CORS Configuration Tight But Frontend URL Hardcoded:**
- Risk: `cmd/server/main.go:141` uses single frontend URL from config. If frontend deployed to different domain, CORS will fail
- Files: `cmd/server/main.go:140-145`
- Current mitigation: AllowCredentials=true with single origin is relatively safe
- Recommendations:
  - Support comma-separated origins list
  - Validate origin against allowlist at request time
  - Document production CORS setup clearly

**Google OAuth Flow Incomplete:**
- Risk: State parameter validation not implemented; vulnerable to CSRF
- Files: `internal/auth/handlers.go:33-39` (unimplemented callback)
- Current mitigation: OAuth endpoints return 501, so vulnerable code is not active
- Recommendations:
  - Implement state parameter generation and validation
  - Use secure cookie for state storage
  - Add PKCE support once implemented

**No Input Length Limits on Task/Subtask Instructions:**
- Risk: User can submit unlimited-size task descriptions; passed to claude CLI without truncation
- Files: `internal/handlers/conversations.go:290-302` (no validation of content length)
- Current mitigation: `internal/handlers/helpers.go:29` limits HTTP body to 1MB
- Recommendations:
  - Add explicit field-level length validation (e.g., task description max 5000 chars)
  - Log suspicious large payloads
  - Consider rate limiting per user

**A2A Protocol Missing Authentication:**
- Risk: `internal/a2a/server.go:81-94` (HandleJSONRPC) is public endpoint with no auth check
- Files: `cmd/server/main.go:154` (route registered without auth middleware)
- Current mitigation: A2A endpoint uses JSON-RPC 2.0 format, but no API key or signature verification
- Recommendations:
  - Add optional JWT or API key validation to A2A requests
  - Implement rate limiting per client IP
  - Log all A2A requests with caller identity

## Performance Bottlenecks

**N+1 Query Pattern in Message Listing:**
- Problem: `internal/handlers/conversations.go:246-283` loads all messages for conversation, but doesn't fetch sender names or task details in same query
- Files: `internal/handlers/conversations.go:242-283`
- Cause: Simple SELECT without JOIN for sender details
- Current impact: Small (sender names loaded separately in frontend), but pattern establishes bad precedent
- Improvement path: Use JOINs to fetch sender user details in single query if needed server-side. Consider pagination.

**Unbounded Message Query Result Set:**
- Problem: `internal/handlers/conversations.go:250` fetches ALL messages for a conversation with no LIMIT or pagination
- Files: `internal/handlers/conversations.go:246-251`
- Cause: No pagination implementation; uses ORDER BY created_at but no cursor
- Impact: Large conversations (1000+ messages) serialize entire history on every fetch, consuming bandwidth and memory
- Improvement path: Add cursor-based pagination (using created_at timestamp as cursor). Implement page size limits. Add client-side infinite scroll.

**Template Lookup During Task Execution is Synchronous:**
- Problem: `internal/executor/executor.go:119-135` loads template synchronously during planning; blocks entire execution
- Files: `internal/executor/executor.go:114-135`
- Cause: Template lookup happens before orchestrator call; if template query is slow, planning is delayed
- Impact: With slow database, task planning time increases linearly with template lookup time
- Improvement path: Pre-load templates at startup or cache them with TTL. Use async template fetch if template is optional.

**Audit Logger Synchronous Blocking Calls:**
- Problem: `internal/executor/executor.go:540,624` (audit logging) perform database writes inline during subtask execution
- Files: `internal/executor/executor.go:540,624` (example lines, many more)
- Cause: All audit calls use `e.Audit.Log()` synchronously
- Impact: Slow database writes block subtask completion processing; can cascade delays across many subtasks
- Improvement path: Implement async audit logging via buffered channel and background writer. Batch audit entries for bulk insert.

**Health Check Polling Every 2 Minutes System-Wide:**
- Problem: `cmd/server/main.go:112-118` spawns health checker with fixed 2-minute interval; no backoff or jitter
- Files: `cmd/server/main.go:116`
- Cause: Single global interval; all agents checked in lockstep
- Impact: Every 2 minutes, all agents receive health check in parallel; creates periodic load spikes
- Improvement path: Add jitter per agent (randomize check time +/- 30s). Implement exponential backoff for unhealthy agents. Make interval configurable.

**Subprocess Spawning for LLM Calls is Serialized:**
- Problem: `internal/orchestrator/orchestrator.go:135-146` each LLM call spawns a new `claude` process
- Files: `internal/orchestrator/orchestrator.go:135`
- Cause: `exec.CommandContext()` blocks until command completes
- Impact: Planning takes 5-10+ seconds per task; multiple concurrent task planning serializes
- Improvement path: Use Anthropic SDK for streaming and connection reuse. Batch multiple prompt calls if possible. Consider connection pooling.

## Fragile Areas

**DAGExecutor.runSubtask is Complex and Tightly Coupled:**
- Files: `internal/executor/executor.go:495-700+` (complex subtask lifecycle)
- Why fragile:
  - Long function (200+ lines) with nested goroutine logic
  - Multiple database operations mixed with LLM/A2A calls
  - Retry logic, backoff, timeout, and cancellation intertwined
  - Hard to test in isolation due to dependencies
- Safe modification: Extract retry/backoff logic into separate Retrier interface. Extract A2A communication into CallAgent adapter function. Add comprehensive logging at each state transition.
- Test coverage: No unit tests for runSubtask; integration tests only

**Orchestrator.Plan LLM Output Parsing is Brittle:**
- Files: `internal/orchestrator/orchestrator.go:58-84` (Plan function)
- Why fragile:
  - Expects strict JSON format from claude CLI; no schema validation
  - If LLM returns markdown-fenced JSON, must strip manually
  - No fallback if response is unparseable
  - Error message truncates to 200 chars, losing context
- Safe modification: Implement strict JSON schema validation using jsonschema library. Add retry loop with different system prompts on failure. Log full response before truncation.
- Test coverage: `internal/orchestrator/orchestrator_test.go` tests happy path only

**Conversations Handler Delete Transaction Has Dependency Ordering:**
- Files: `internal/handlers/conversations.go:183-240` (Delete function)
- Why fragile:
  - Multiple DELETE statements in specific order (events → subtasks → messages → tasks → conversation)
  - If order changes, foreign key constraints may cause failures
  - No schema-level cascading; manual ordering required
  - Easy to accidentally reorder and break
- Safe modification: Use database-level CASCADE ON DELETE in schema instead of manual transaction ordering. Or document exact dependency graph in comments.
- Test coverage: `internal/handlers/conversations_test.go` does test deletion, but only of empty conversations

**A2A Protocol Message Part Flattening is Lossy:**
- Files: `internal/handlers/conversations.go:316-322` (mention regex) and throughout message handling
- Why fragile:
  - Multiple message parts (text, data, artifacts) must be combined into single content string
  - Regex-based mention extraction assumes specific format `<@id|name>`
  - No validation that ID extracted is valid agent ID
  - Easy to introduce parsing bugs if mention format changes
- Safe modification: Store message parts as structured data (JSON array) instead of flattened string. Validate extracted agent IDs against agent table. Add comprehensive mention parsing tests.
- Test coverage: No unit tests for mention extraction logic

## Scaling Limits

**Single PostgreSQL Database Connection Pool:**
- Current capacity: pgx pool uses default 25 connections
- Limit: With 100+ concurrent requests (DAG executor goroutines), connection pool exhaustion likely
- Impact: Requests queue on database; task execution becomes single-threaded at database level
- Scaling path:
  1. Increase pool size (e.g., 50-100 connections) based on load testing
  2. Implement query result caching for frequently accessed data (agent list, agent health)
  3. Consider database read replicas for analytics queries
  4. Monitor connection pool utilization and alert at 80%

**Unbounded Goroutine Creation from DAG Execution:**
- Current capacity: System can run ~10,000 goroutines before memory pressure
- Limit: With 1000 subtasks and 10 concurrent per agent, 1000+ goroutines spawned
- Impact: Memory usage grows; scheduling overhead increases; GC pauses lengthen
- Scaling path:
  1. Implement worker pool with bounded goroutine count (e.g., 50-100 global)
  2. Use semaphore-based concurrency control (already partially implemented)
  3. Monitor goroutine count; alert if exceeds threshold
  4. Implement task queue instead of direct goroutine spawning

**SSE Broker Memory Footprint with Many Clients:**
- Current capacity: `internal/events/broker.go` keeps in-memory channel per subscriber
- Limit: With 1000 clients watching tasks, 1000 buffered channels (default size 10 events) = unbounded memory
- Impact: Server memory grows with connected client count; disconnects may not clean up channels
- Scaling path:
  1. Implement channel cleanup on client disconnect
  2. Add configurable buffer size with sensible defaults
  3. Consider moving to Redis pub/sub for distributed deployments
  4. Implement client connection limits

**Task Plan JSON Size Unbounded:**
- Current capacity: Tasks store entire execution plan as JSON; no size limit
- Limit: With 10,000-step task plan, JSON can be multi-MB
- Impact: Database storage grows; plan JSON serialization/deserialization costs increase; network transfer slows
- Scaling path:
  1. Add max plan size limit (e.g., 10,000 subtasks)
  2. Compress plan JSON before storage
  3. Implement plan delta/versioning to avoid storing full plan for each replan
  4. Consider separate plan storage (blob store) for large plans

## Dependencies at Risk

**orchestrator: claude CLI Dependency:**
- Risk: External process dependency; breaks if not installed or if CLI changes
- Impact: Task planning fails; orchestrator becomes unavailable
- Migration plan: Implement Anthropic Go SDK integration as primary, CLI as fallback

**auth: No PKCE for OAuth (Future Risk):**
- Risk: If OAuth implementation added without PKCE, vulnerable to authorization code interception
- Impact: OAuth tokens could be stolen via man-in-the-middle
- Migration plan: Design OAuth implementation with PKCE from the start

## Missing Critical Features

**Expired Session Cleanup Missing:**
- Problem: `DeleteExpired()` method exists but is never called; expired sessions accumulate forever
- Files: `internal/auth/session.go:55-57`
- Blocks: Long-term production deployments will have database bloat
- Fix: Add scheduled background job (e.g., via `time.Ticker`) to delete expired sessions daily

**No Request Rate Limiting:**
- Problem: API endpoints have no rate limiting; single user can spam requests
- Blocks: Production deployments vulnerable to abuse
- Fix: Add middleware using `github.com/go-chi/rate` or similar

**No Dead Letter Queue for Failed Subtasks:**
- Problem: Failed subtasks that exceed max attempts are deleted; no audit trail
- Files: `internal/executor/executor.go:972,977`
- Blocks: Cannot investigate why subtasks failed after deletion
- Fix: Move failed subtasks to separate `failed_subtasks` table for later analysis

**No Audit Log Retention Policy:**
- Problem: Audit logs grow indefinitely with no cleanup
- Files: `internal/audit/logger.go` (implied)
- Blocks: Database storage grows unbounded
- Fix: Implement archival and deletion of audit logs older than N days

**No Task Timeout Enforcement:**
- Problem: Tasks can run forever if agents are unresponsive
- Files: `internal/executor/executor.go` (runDAGLoop has no task-level timeout)
- Blocks: Orphaned long-running tasks consume resources
- Fix: Add configurable per-task timeout; fail task if execution exceeds limit

**Google OAuth Not Implemented:**
- Problem: OAuth endpoints return 501 Not Implemented
- Files: `internal/auth/handlers.go:29,35`
- Blocks: Production multi-user deployments cannot use SSO
- Fix: Implement full OAuth 2.0 flow with state validation

## Test Coverage Gaps

**DAGExecutor.runSubtask Critical Path Not Unit Tested:**
- What's not tested:
  - Subtask retry logic with exponential backoff
  - A2A message send and status polling loop
  - Error handling and failure transitions
  - Concurrent execution limits per agent
- Files: `internal/executor/executor.go:495-700+`
- Risk: Changes to subtask execution can introduce race conditions or deadlocks
- Recommended: Add unit tests with mock A2A client and mock database

**Orchestrator.Plan JSON Parsing Not Stress Tested:**
- What's not tested:
  - Malformed JSON responses from claude CLI
  - Very large execution plans (1000+ subtasks)
  - LLM output with markdown fences
  - Non-deterministic response formats
- Files: `internal/orchestrator/orchestrator.go:58-84`
- Risk: Production failures due to unexpected LLM output formats
- Recommended: Add fuzzing tests with real claude CLI responses

**Conversations Handler Concurrent Message Sending:**
- What's not tested:
  - Two concurrent SendMessage requests on same conversation
  - Race condition in message insertion
  - Mention extraction with special characters
  - Concurrent conversation deletion while message sends in progress
- Files: `internal/handlers/conversations.go:290-350+`
- Risk: Message ordering corruption or lost mentions
- Recommended: Add concurrent load tests

**A2A Server Recovery Path:**
- What's not tested:
  - Server restart during subtask execution
  - Agent unreachable during recovery
  - Recovery with mixed task states
  - Recovery with many orphaned subtasks
- Files: `internal/executor/recovery.go:26-180`
- Risk: Tasks stuck in running state after restart; manual database cleanup required
- Recommended: Add integration test that crashes server mid-execution and verifies recovery

**Frontend Type Sync with Go Models:**
- What's not tested:
  - Go model changes are reflected in TypeScript interfaces
  - JSON serialization matches expected snake_case format
  - All Go struct fields have corresponding TypeScript fields
- Files: `web/lib/types.ts` vs `internal/models/models.go`
- Risk: Go API changes silently break frontend; runtime TypeScript errors
- Recommended: Add schema validation test that compares Go JSON output with TypeScript interfaces

**Session Expiration Edge Cases:**
- What's not tested:
  - Concurrent session validation and deletion
  - Session cookie corruption
  - Session TTL boundary conditions
  - DeleteExpired cleanup correctness
- Files: `internal/auth/session.go`
- Risk: Session validation bugs could allow unauthorized access
- Recommended: Add unit tests for session store with controlled time

**No Integration Tests for Full Task Execution Flow:**
- What's not tested:
  - Complete task from creation → planning → execution → completion
  - Real orchestrator (claude CLI) output handling
  - Multi-agent collaboration on single task
  - Replanning after agent failure
- Files: Entire `internal/executor` and `internal/orchestrator` packages
- Risk: End-to-end workflows may fail in production despite unit test coverage
- Recommended: Add integration tests with test agents and mock claude CLI

---

*Concerns audit: 2026-04-04*
