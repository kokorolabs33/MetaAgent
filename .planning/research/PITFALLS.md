# Domain Pitfalls

**Domain:** A2A meta-agent platform — real-time observability, chat interaction, multi-task views, templates
**Project:** TaskHub
**Researched:** 2026-04-04
**Milestone scope:** Agent status visualization, enhanced chat intervention, multi-task parallel view, task templates

---

## Critical Pitfalls

Mistakes that cause rewrites, data loss, or demo-breaking bugs.

---

### Pitfall 1: SSE Subscribe-Before-Replay Race Window

**What goes wrong:** The current `stream.go` subscribes to the broker AFTER replaying historical events from the database. Any event published between the DB read completing and the broker subscription being registered is silently lost. On a busy task this window is real: fast-executing subtasks can complete and publish while the SELECT is still running.

**Why it happens:** The natural sequence is "query DB, then subscribe." But events flow continuously from the executor, so this ordering creates a gap.

**Evidence in codebase:** `internal/handlers/stream.go:53-74` — `ListByTask` runs, then `Broker.Subscribe` is called. If a `subtask.completed` event fires between those two lines, the client never sees it and the DAG shows a wrong state until the next full page refresh.

**Consequences:** DAG nodes frozen in "running" state on the frontend even though execution completed. Users see stale status until they reload.

**Prevention:** Subscribe to the broker first, then replay from DB, then start forwarding live events (draining any that arrived during replay by deduplicating on event ID). The `Last-Event-ID` mechanism already exists for reconnection — extend the same dedup logic to initial load.

**Detection warning signs:**
- DAG node stuck in "running" state when terminal events are in the DB
- `subtask.completed` events visible in the audit log but not rendered

**Phase:** Should be addressed in the agent status visualization phase before adding more status granularity.

---

### Pitfall 2: Multi-Task Parallel View Hits the HTTP/1.1 SSE 6-Connection Limit

**What goes wrong:** A multi-task dashboard view that opens one `EventSource` per visible task will exhaust the browser's per-domain connection limit (6 under HTTP/1.1) once 3-4 tasks are shown simultaneously. Additional tasks silently get no updates, or earlier connections start timing out. This limit is per browser, not per tab — it applies globally.

**Why it happens:** Browsers enforce this at the network layer. Chrome and Firefox have both marked the increase request as "Won't Fix" for HTTP/1.1. The limit is well-documented but easy to miss when building the feature in dev where only 1-2 tasks are typically open.

**Evidence in codebase:** `web/lib/sse.ts` opens a separate `EventSource` per task ID. The current single-task view is fine; a multi-task dashboard view would immediately be affected.

**Consequences:** Multi-task view works for 2-3 tasks in dev, breaks silently for 4+ in production/demos. SSE connections queue behind each other; tasks show stale status or miss events entirely.

**Prevention — two options (pick one before building the feature):**

Option A: Serve the backend over HTTP/2. Go's `net/http` supports HTTP/2 automatically when TLS is configured; under HTTP/2 the browser multiplexes all SSE streams over a single TCP connection (default 100 streams). For a dev/demo deployment this is the cleanest fix.

Option B: Multiplex SSE at the application layer. Replace per-task `EventSource` with a single `/api/events/stream?tasks=id1,id2,id3` endpoint. The server fans in events from multiple broker subscriptions and tags each event with `task_id`. The frontend routes events to the right store slice by `task_id`.

**Detection warning signs:**
- Multi-task view works in dev but hangs/freezes in a browser with >2 tabs open
- Network inspector shows SSE connections in "pending" state

**Phase:** Must be resolved before multi-task parallel view is built — it is a fundamental architecture decision for that feature.

---

### Pitfall 3: Agent Status "Online" Indicator Shows Stale State

**What goes wrong:** The `is_online` and `last_health_check` fields in the `agents` table are written by the health poller every 2 minutes (`cmd/server/main.go:116`). An agent that becomes unreachable between poll cycles shows as "online" in the UI for up to 2 minutes. During a live demo with multiple agents, this creates a confusing situation: the DAG shows a subtask failing while the agent status indicator glows green.

**Why it happens:** Health status is pulled on a fixed schedule, not event-driven. The polling interval was hardcoded without considering the user-observable staleness window. The concern is noted in `CONCERNS.md` ("Health Check Polling Every 2 Minutes System-Wide").

**Evidence in codebase:** `cmd/server/main.go:112-118` hardcodes 2-minute interval. `internal/handlers/agent_health.go:28-33` serves `is_online` directly from the DB without any staleness annotation.

**Consequences:** Agent status badge shows green while the agent is actually dead. Users cannot distinguish "agent is healthy" from "we haven't checked yet."

**Prevention:**
- Add a `staleness_threshold_sec` concept: if `now - last_health_check > threshold`, treat status as "unknown" not "online" in the API response (or in the frontend rendering logic)
- Add jitter to the health poll so agents don't all flip at the same moment
- When a subtask fails with an agent-unreachable error, proactively mark that agent `is_online = false` in the same transaction — don't wait for the next poll cycle

**Detection warning signs:**
- Agent shows "online" in the status panel but has `last_health_check` > 3 minutes ago
- Subtask failure events cite agent unreachability while the health overview still shows green

**Phase:** Agent status visualization phase — bake staleness handling in from day one, not as a follow-up.

---

### Pitfall 4: Chat Intervention Into a Running Subtask Produces Silent Race

**What goes wrong:** When a user sends a chat message to intervene in an in-progress subtask, the message arrives in the conversation while the executor is simultaneously running `runSubtask` for that subtask. Both paths attempt to update subtask state and publish SSE events. The executor's state machine does not check for user intervention messages; it overwrites whatever the user communicated.

**Why it happens:** The executor loop (`internal/executor/executor.go:495-700`) polls the A2A agent for task completion and updates subtask status autonomously. There is no shared mutex or interruption signal between the chat send path (`internal/handlers/conversations.go`) and the executor loop. The CONCERNS document already flags `runSubtask` as "complex and tightly coupled."

**Evidence in codebase:** `internal/handlers/conversations.go:290-350` sends messages via DB insert + SSE publish. `internal/executor/executor.go` runs in a goroutine that reads A2A responses and writes subtask state. No coordination between them exists.

**Consequences:**
- User's intervention message is delivered to the channel but the running subtask ignores it
- If the frontend shows an "Awaiting intervention" state, it may never clear
- Concurrent DB writes to the subtask row can produce inconsistent status

**Prevention:**
- Define a clear intervention contract: user chat messages during execution are "advisory" (visible in channel but don't interrupt execution) vs "directive" (pause the executor, inject input, resume). These require fundamentally different backend plumbing.
- For advisory chat: the current architecture works — accept the limitation and document it in the UI ("message delivered; agent will see it on next turn")
- For directive intervention: add an `input_queue` channel to the executor's subtask context, and have the chat handler push to it; the executor polls this channel inside its loop alongside the A2A response poll

**Detection warning signs:**
- User sends a clarification message, subtask completes with old output anyway
- `input_required` subtask state appears but chat messages don't unblock it

**Phase:** Enhanced chat interaction phase — decide the intervention model (advisory vs directive) before writing a single line of code. The wrong choice leads to a rewrite.

---

### Pitfall 5: Template Steps Capture Specific Agent IDs, Breaking Reuse

**What goes wrong:** The `CreateFromTask` handler (`internal/handlers/templates.go:230-320`) copies subtask instructions verbatim from the completed task's plan. If the LLM-generated plan embedded references to specific agent UUIDs in the instruction text (e.g., "ask agent `3e7f...` to review"), those UUIDs are baked into the template. On reuse, those agents may not exist, may be offline, or may be different agents entirely.

More broadly, templates that were captured from a very specific task context ("Write a REST API for the Acme Corp customer portal") will fail when applied to a different context ("Write a REST API for Globex Corp") unless the instructions are parameterized. The current template schema has a `variables` field but `CreateFromTask` populates it as `[]` — it never extracts variables from the instruction text.

**Why it happens:** Template extraction is implemented as a structural copy of the DAG, not a semantic parameterization of it. This is the natural MVP shortcut, but it produces templates that are single-use in practice.

**Evidence in codebase:** `internal/handlers/templates.go:263-277` — `instruction_template` field is set to `st.Instruction` verbatim. `variables` is initialized to `[]` unconditionally.

**Consequences:**
- Templates appear to be reusable but silently produce identical tasks with hardcoded context
- The "experience accumulation" value proposition is undermined if templates cannot generalize
- Template versions proliferate as users try to manually edit instructions, creating maintenance burden

**Prevention:**
- Before investing in a rich template UI, decide what "template variable" means: named placeholders like `{{project_name}}` vs user-prompted variables at task creation time vs LLM-derived substitution at planning time
- Implement a lightweight variable extraction step at template creation: scan instruction text for obvious specifics (project names, URLs, entity names) and offer to convert them to `{{variable_name}}` placeholders
- Document the current limitation prominently in the UI ("Template captures structure; review instructions before reuse")

**Detection warning signs:**
- Users report all tasks created from a template produce the same instructions as the original task
- Templates list shows many near-duplicate entries with slight instruction variations

**Phase:** Task templates phase — the variable extraction design must happen before building the frontend StepEditor; retrofitting it after the fact requires a schema migration and UI overhaul.

---

## Moderate Pitfalls

Mistakes that degrade quality or create tech debt, but do not require rewrites.

---

### Pitfall 6: Zustand Event Handler Captures Stale Store Snapshot

**What goes wrong:** The `connectSSE` function in `store.ts` captures `get().handleEvent` as a closure at subscription time. If the store's `handleEvent` method references state that changes after subscription (e.g., `currentTask` is replaced), the closure holds stale values. For the multi-task parallel view, this is more acute: each task's SSE connection must dispatch to the right slice of state, but a shared store makes this fragile.

**Why it happens:** Zustand's `get()` is called lazily inside event callbacks, which is correct for the current single-task view. But `connectSSE` passes a bound callback: `(event) => get().handleEvent(event as ...)` — this is fine as long as `handleEvent` itself always calls `get()` internally. The risk is subtle: any `set()` inside `handleEvent` that reads state from the closure (not from `get()`) can capture a stale snapshot.

**Evidence in codebase:** `web/lib/store.ts:135-141` — the SSE connection passes an arrow function that calls `get().handleEvent`. The `handleEvent` function at line 151 reads `const { currentTask } = get()` which is correct. The pitfall is latent: any future developer who refactors `handleEvent` to accept `currentTask` as a parameter (for performance) will introduce the bug.

**Consequences:** Events for task A are processed against the state of task B when the user navigates between tasks quickly. Subtask status updates land on the wrong task in the UI.

**Prevention:**
- Add a task-ID guard at the top of `handleEvent`: if `event.task_id !== currentTask?.id`, drop the event (or route it to the correct task store slice)
- For the multi-task view, use a `Map<taskId, taskState>` store shape rather than a single `currentTask`
- Document the `get()` vs closure rule in a comment next to `handleEvent`

**Detection warning signs:**
- Rapidly switching between task detail views causes one task's events to appear in another
- DAG nodes update for a task that is not currently displayed

**Phase:** Multi-task parallel view phase — restructure the store shape before adding the second task pane, not after.

---

### Pitfall 7: SSE Broker Channel Drop Under Load Erodes Observability

**What goes wrong:** The broker (`internal/events/broker.go:52-68`) silently drops events when a subscriber's channel buffer (64 events) is full. The comment "they can catch up from DB" is correct for reconnection scenarios, but the frontend's `EventSource` does not automatically re-fetch state on drop — it only replays from `Last-Event-ID` on reconnection. A slow client that can't process 64 events fast enough will silently miss status updates without triggering a reconnect.

**Why it happens:** The drop-on-full design is intentional for backpressure, but the implicit assumption is that the client reconnects and replays. In practice, the `EventSource` only reconnects on TCP disconnect — a full channel that never closes the connection does not trigger reconnect.

**Evidence in codebase:** `internal/events/broker.go:61-67` — `select { case ch <- event: default: }`. No drop counter, no log line.

**Consequences:** During heavy subtask execution (many parallel agents writing events fast), the frontend progressively falls behind real state without notifying the user. The DAG appears partially updated.

**Prevention:**
- Add a drop counter per subscriber; if drops exceed N within a window, proactively close the SSE connection so the client reconnects and replays from `Last-Event-ID`
- Or: increase the buffer size from 64 to 256 for the agent-status use case where burst events are expected
- Add a log line on every drop for observability: `log.Printf("broker: dropped event %s for subscriber (channel full)", event.ID)`

**Detection warning signs:**
- Frontend DAG shows mixed states (some subtasks completed, some stuck at prior state) during heavy execution
- No `Last-Event-ID` reconnect traffic visible in network inspector despite events having been published

**Phase:** Agent status visualization phase — test with simulated event bursts before declaring the feature complete.

---

### Pitfall 8: Template Execution Tracking Silently Fails

**What goes wrong:** CONCERNS.md documents that template version tracking inside the executor (`internal/executor/executor.go`) uses `_ =` to discard DB errors. The `template_executions` table — which records outcome, replan count, and HITL interventions per execution — is the core data source for "experience accumulation." If those inserts silently fail, the template improvement loop has no data.

**Why it happens:** The broader pattern of silenced DB errors (documented across ~18 locations in `executor.go`) was carried forward to the template tracking code without special-casing it.

**Evidence in codebase:** CONCERNS.md `Database Query Errors Silently Ignored`, specifically `internal/executor/executor.go:86,125,141,166,194,421,...`. Template version tracking is among the affected operations.

**Consequences:** The "experience accumulation" feature ships but accumulates nothing. Template execution history is empty. No data feeds template improvement recommendations.

**Prevention:**
- Dedicate a focused error-handling pass to the template execution recording paths specifically
- These writes are critical for the template feature's value proposition — they should log errors at minimum, or fail the template recording atomically (without failing the task)
- Add an integration test that verifies a completed task writes a `template_executions` row when a template was used

**Detection warning signs:**
- `template_executions` table is empty after running tasks from templates
- `ListExecutions` endpoint returns `[]` for templates that have been used

**Phase:** Task templates phase — fix the silent error pattern for template-related DB writes before building the execution history UI.

---

### Pitfall 9: GitHub Open-Source Demo Fails on First Clone Due to Environment Coupling

**What goes wrong:** The platform's core LLM integration uses `exec.CommandContext()` to spawn the `claude` CLI (documented in CONCERNS.md). A developer who clones the repo to explore the A2A reference implementation must have the Claude CLI installed, authenticated, and in `$PATH` — before they can run a single task. There is no graceful degradation, mock mode, or clear error message if the CLI is absent.

**Why it happens:** The CLI dependency is a known MVP shortcut. For internal development it is acceptable. For open-source readiness, it is a first-run blocker for the majority of developers.

**Evidence in codebase:** CONCERNS.md "orchestrator: claude CLI Dependency." `internal/orchestrator/orchestrator.go:135-146`.

**Consequences:** GitHub stars do not convert to "I ran it" experiences. Developers hit a confusing failure (likely a cryptic exec error or a task stuck in "planning") before seeing any agent coordination. The demo value is zero for anyone without the CLI set up.

**Prevention:**
- Add a startup check: if the `claude` binary is not found, print a clear setup instruction and optionally exit with an actionable error rather than silently failing at task creation time
- Add a `--mock-llm` flag or `MOCK_LLM=true` env var that makes the orchestrator return a hardcoded demo plan, enabling the UI/SSE/DAG flow to be explored without any LLM API key
- Document the dependency prominently in the README with a setup checklist

**Detection warning signs:**
- Tasks created via the UI get stuck in "planning" status forever
- No error visible in the UI; error only appears in server logs as an exec failure

**Phase:** GitHub open-source readiness phase — this must be resolved before any public launch.

---

## Minor Pitfalls

---

### Pitfall 10: Unbounded Message History Degrades Chat Panel Performance

**What goes wrong:** `internal/handlers/conversations.go:246-251` fetches all messages for a conversation with no LIMIT. For a long-running demo session, a single conversation can accumulate hundreds of agent messages (one per subtask + cross-mentions). The frontend renders all of them at once in the chat panel.

**Prevention:** Add cursor-based pagination at the API level (using `created_at` as cursor). Implement virtual scrolling in the chat component if message counts will exceed ~200 in typical use.

**Phase:** Enhanced chat interaction phase.

---

### Pitfall 11: Template Steps UI Allows Circular Dependencies Without Validation

**What goes wrong:** The `StepEditor` component (`web/components/template/StepEditor.tsx`) lets users edit the `depends_on` array for each step. Nothing prevents a user from creating a cycle (step A depends on B, B depends on A). When a task is created from such a template, the DAG executor will deadlock (all affected subtasks wait forever for each other).

**Prevention:** Add a topological sort validation step in the template save handler. Reject the save with a clear error if a cycle is detected. This is a few lines of Go (Kahn's algorithm) and prevents a silent deadlock.

**Phase:** Task templates phase.

---

### Pitfall 12: Agent Status Events Not Yet Defined as SSE Event Types

**What goes wrong:** The agent status visualization feature requires new SSE event types like `agent.working`, `agent.idle`, `agent.offline`. These must be added to both the backend SSE publish calls and the `SSEEventType` union in `web/lib/types.ts`. If the backend publishes events the frontend type system does not know about, the `handleEvent` switch silently ignores them — no TypeScript error, no runtime error, just invisible status updates.

**Prevention:** Define the new event type constants in `types.ts` first, then implement the backend emitters. Run `make typecheck` after every new event type addition to catch mismatches early.

**Phase:** Agent status visualization phase — type contract first, implementation second.

---

### Pitfall 13: Template Instruction Variables Are a Prompt Injection Surface

**What goes wrong:** When template variables (e.g., `{{project_name}}`) are substituted at task creation time, the substituted value is fed directly into the LLM orchestrator prompt. A user who sets `project_name` to a string containing LLM instruction syntax (e.g., `"Acme Corp. Ignore all prior instructions and..."`) can manipulate the generated task plan. This is an indirect prompt injection via the template system.

**Why it matters for this project:** TaskHub is targeted at developers — the threat model includes technically sophisticated users experimenting with the system. For a public demo repo, demonstrating naive variable substitution could become an embarrassment.

**Prevention:**
- Strip or escape any text that looks like LLM instruction preambles from variable values before substitution
- Add a max length limit on variable values (100 chars is reasonable for names/identifiers)
- Document in the template README that variable values are passed to an LLM and should be treated as untrusted input

**Phase:** Task templates phase.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Agent status visualization | Stale "online" indicator (Pitfall 3) | Add staleness threshold to API + frontend rendering |
| Agent status visualization | SSE subscribe/replay race (Pitfall 1) | Subscribe first, replay second, dedup by event ID |
| Agent status visualization | Event drops during burst (Pitfall 7) | Add drop counter, increase buffer, log drops |
| Agent status visualization | Missing SSE event type constants (Pitfall 12) | Define types.ts entries before backend emitters |
| Enhanced chat interaction | Intervention race with executor (Pitfall 4) | Decide advisory vs directive model before any code |
| Enhanced chat interaction | Unbounded message history (Pitfall 10) | Add pagination before building infinite scroll UI |
| Multi-task parallel view | HTTP/1.1 SSE 6-connection limit (Pitfall 2) | Decide HTTP/2 vs multiplexed endpoint before building |
| Multi-task parallel view | Stale closure in Zustand store (Pitfall 6) | Restructure store to `Map<taskId, state>` shape |
| Task templates | Template captures specific context not structure (Pitfall 5) | Design variable extraction before building StepEditor |
| Task templates | Silent template execution tracking failure (Pitfall 8) | Fix silenced DB errors for template recording paths |
| Task templates | Circular dependency in step editor (Pitfall 11) | Add DAG cycle detection in template save handler |
| Task templates | Prompt injection via template variables (Pitfall 13) | Add variable length limits + input sanitization |
| GitHub open-source readiness | First-clone failure due to CLI dependency (Pitfall 9) | Add startup check + mock LLM mode |

---

## Sources

- Codebase analysis: `internal/events/broker.go`, `internal/handlers/stream.go`, `internal/handlers/templates.go`, `internal/executor/executor.go`, `web/lib/store.ts`, `web/lib/sse.ts`
- Project context: `.planning/PROJECT.md`, `.planning/codebase/CONCERNS.md`
- A2A Protocol Specification — event ordering and streaming requirements: https://a2a-protocol.org/latest/specification/
- Browser SSE 6-connection limit (Chrome bug tracker, marked Won't Fix): https://issues.chromium.org/issues/40329530
- SSE HTTP/2 multiplexing solution: https://medium.com/@kaitmore/server-sent-events-http-2-and-envoy-6927c70368bb
- Multi-agent failure modes (error propagation, race conditions): https://medium.com/@rakesh.sheshadri44/the-dark-psychology-of-multi-agent-ai-30-failure-modes-that-can-break-your-entire-system-023bcdfffe46
- LLM agent prompt injection via workflow templates (OWASP LLM01:2025): https://genai.owasp.org/llmrisk/llm01-prompt-injection/
- SSE event deduplication on reconnect: https://tigerabrodi.blog/server-sent-events-a-practical-guide-for-the-real-world
- Context window overflow in multi-agent systems: https://redis.io/blog/context-window-overflow/
