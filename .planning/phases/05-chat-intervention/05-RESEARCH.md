# Phase 5: Chat Intervention - Research

**Researched:** 2026-04-04
**Domain:** Real-time advisory messaging to sub-agents during task execution
**Confidence:** HIGH

## Summary

Phase 5 adds advisory @mention messaging to active sub-agents during task execution. The user sends a message with `@AgentName` in the chat input, the backend validates the agent has an active subtask, dispatches the message via A2A protocol in a completely isolated goroutine, and publishes the response as a chat message with an "Advisory reply" visual distinction. The entire flow is fire-and-forget from the executor's perspective -- advisory calls never modify subtask status, never cancel the DAG, and never block the execution loop.

The codebase already has approximately 80% of the infrastructure needed. The `MessageHandler.routeToAgents` method already parses @mentions and dispatches via `SendFollowUp` in detached goroutines. The `GroupChat.tsx` component already has `showMentions` autocomplete state. The `AgentStatusDot` component from Phase 2 is ready for reuse. The primary engineering work is: (1) a new `SendAdvisory` executor method that sends A2A messages without touching subtask status, (2) SSE typing indicators, (3) frontend autocomplete enrichment with agent status, and (4) advisory reply visual distinction in ChatMessage.

**Primary recommendation:** Create a new `SendAdvisory` method on `DAGExecutor` that is completely independent from `SendFollowUp`. Do NOT modify `SendFollowUp` -- it has correct behavior for the input_required workflow. The advisory method should use a separate `context.WithTimeout` (60s), publish typing/response events, and never write to the subtasks table.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Advisory-only model -- user messages are informational, agents may or may not act on them (directive mode deferred to v2)
- **D-02:** When user @mentions an agent with no active (running/input_required) subtask, block the send and show inline error: "Agent X is not currently executing -- advisory messages can only be sent to active agents"
- **D-03:** Multiple @mentions in a single message route independently -- each mentioned agent receives the full message and responds independently, responses appear in conversation flow as they arrive
- **D-04:** Advisory message context sent to A2A agent: user message text + current subtask description + subtask's previous output (so agent understands what it's working on)
- **D-05:** Messages routed via existing `SendFollowUp` mechanism in a detached goroutine with background context
- **D-06:** Agent advisory replies use existing ChatMessage component but with a subtle visual distinction -- small "Advisory reply" label or different background tint to differentiate from normal agent execution messages
- **D-07:** Show "Agent X is typing..." indicator in the conversation flow after user sends an advisory @mention, until the response arrives or timeout
- **D-08:** User's advisory messages display like normal user messages with @mention highlighting -- no special advisory tag on the user side
- **D-09:** Typing indicator implemented via SSE event (`agent.typing` on the task's channel) -- published when follow-up starts, cleared when response arrives or times out
- **D-10:** @mention autocomplete list shows all agents that have participated in the current task (any agent with at least one subtask), with status indicators showing which are currently active (running/input_required) vs completed/pending
- **D-11:** Agents not currently active appear grayed out in the autocomplete with "(not active)" label -- selecting them triggers the D-02 block behavior
- **D-12:** After sending, user message appears immediately in conversation flow, then typing indicator starts -- clear feedback loop
- **D-13:** Existing `showMentions` and `mentionFilter` state in GroupChat.tsx extended (not replaced) with status-aware filtering
- **D-14:** Advisory A2A calls run with 60-second timeout, automatic retry once on failure, then show "Agent X did not respond" error message in conversation flow
- **D-15:** Complete isolation -- advisory calls run in independent goroutines, failures only produce error messages in conversation, executor continues unaffected regardless of advisory outcome
- **D-16:** Advisory follow-up uses a separate A2A context (not the executor's main context) to prevent cancellation propagation -- if the task completes or is canceled while an advisory is in-flight, the advisory goroutine still completes gracefully

### Claude's Discretion
- Exact "Advisory reply" label text and styling (color, border, background)
- Typing indicator animation implementation (dots, spinner, etc.)
- Whether to cap the number of concurrent advisory calls per task
- Error message wording for timeout/failure scenarios
- Subtask context truncation strategy if previous output is very long

### Deferred Ideas (OUT OF SCOPE)
- **Dynamic agent invitation:** User wants to @mention agents not participating in the current task and have them join. This requires modifying the executor's DAG to dynamically create subtasks -- a significant architectural change. Belongs in a future phase.
- **Directive mode:** Sending commands that directly affect agent behavior (e.g., "stop", "change approach") -- deferred to v2 per STATE.md decision.
- **Multi-turn advisory conversations:** Threaded back-and-forth between user and agent within an advisory context -- Phase 5 is single-round advisory only.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| INTR-01 | User can send messages to specific sub-agents during active task execution via @mention in chat (advisory mode) | Full architecture documented: new `SendAdvisory` method on executor, `routeToAgents` refactor in MessageHandler, typing indicator SSE events, autocomplete enrichment with AgentStatusDot, ChatMessage advisory label |
</phase_requirements>

## Standard Stack

### Core (already in project)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| chi/v5 | 5.2.5 | HTTP routing | Already used for all API routes [VERIFIED: go.mod] |
| pgx/v5 | 5.9.1 | PostgreSQL driver | Already used for all DB operations [VERIFIED: go.mod] |
| google/uuid | 1.6.0 | UUID generation | Already used for message/event IDs [VERIFIED: go.mod] |
| Next.js | 16.1.6 | Frontend framework | Already in use [VERIFIED: web/package.json] |
| Zustand | 5.0.11 | State management | Already used for task/agent stores [VERIFIED: web/package.json] |
| shadcn/ui | (components) | UI components | Already used for buttons, inputs [VERIFIED: web/components/ui/] |

### Supporting (already in project)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Lucide React | 0.577.0 | Icons | Typing indicator dots, status icons [VERIFIED: web/package.json] |
| Tailwind CSS | v4 | Styling | Advisory reply styling, typing animation [VERIFIED: web/package.json] |

**No new dependencies required.** This phase uses only existing project infrastructure.

## Architecture Patterns

### Backend: Advisory Message Flow

```
User sends POST /api/tasks/{id}/messages with @mention
  |
  v
MessageHandler.Send (existing)
  |-- Save message to DB
  |-- Publish "message" SSE event
  |-- Extract mentions via mentionRe
  |-- For each mention:
  |     |-- Query subtask status for agent
  |     |-- If active (running/input_required):
  |     |     |-- go SendAdvisory(taskID, ...)  // NEW METHOD — no ctx param
  |     |-- If not active:
  |           |-- Return error in response (D-02)
  |
  v
SendAdvisory (new, on DAGExecutor — no ctx parameter, creates own background context)
  |-- Build context: user message + subtask description + previous output
  |-- advCtx = context.WithTimeout(context.Background(), 60s)
  |-- Publish "agent.typing" via Broker-only (transient)
  |-- A2AClient.SendMessage(advCtx, ...) with advisory contextID
  |-- If "working" state: pollUntilTerminal(advCtx, ...) with 60s limit
  |-- On success: publishAdvisoryMessage with advisory metadata
  |-- On failure: retry once, then publish error message
  |-- Always (defer): publish "agent.typing_stopped" via Broker-only (transient)
```

### Critical Design Decision: SendAdvisory vs SendFollowUp

The existing `SendFollowUp` method (executor.go:792) **MUST NOT** be used for advisory messages. [VERIFIED: codebase analysis]

**Why:** `SendFollowUp` modifies subtask status on completion:
- Line 833: `UPDATE subtasks SET status = 'completed', output = $1, completed_at = $2`
- Line 859: publishes `"subtask.completed"` event
- Line 862: `UPDATE subtasks SET status = 'input_required'`

An advisory reply must NEVER change the subtask's status. The agent is still executing its original task; the advisory is a side conversation. Changing status would crash the executor's DAG loop.

**Solution:** New `SendAdvisory` method that:
1. Takes NO ctx parameter — creates its own `context.WithTimeout(context.Background(), 60s)` (D-16)
2. Sends A2A message (reuses same `A2AClient.SendMessage`)
3. Publishes response as a chat message with `metadata: {"advisory": true}`
4. Never touches the subtasks table
5. Publishes typing indicators via Broker-only (transient, not persisted)

### D-05 Reinterpretation

D-05 says "Messages routed via existing `SendFollowUp` mechanism." Based on codebase analysis, this should be interpreted as "use the same A2A communication pattern" (detached goroutine + background context + A2AClient.SendMessage), NOT literally calling the `SendFollowUp` method. The new `SendAdvisory` reuses the same infrastructure but avoids the subtask-modifying side effects.

### Frontend: Typing Indicator Pattern

```
SSE event: { type: "agent.typing", data: { agent_id, agent_name, task_id } }
  |
  v
Zustand store: typingAgents: Record<string, string>  // agent_id -> agent_name
  |
  v
GroupChat.tsx renders typing indicators above input area
  |
  v
SSE event: { type: "agent.typing_stopped", data: { agent_id } }
  |
  v
Store removes agent from typingAgents map
```

### Frontend: Autocomplete Enrichment Pattern

```
GroupChat props: agents: { id, name }[] + subtasks?: SubTask[]
  |
  v
Derive agentWithStatus from subtasks:
  - If subtasks is undefined or empty: show all agents, assume active (fallback)
  - If subtasks loaded: for each agent, find latest subtask
    - status = "running" or "input_required" -> active
    - else -> inactive (grayed out in dropdown)
  |
  v
AgentStatusDot next to agent name in dropdown
  |
  v
On select inactive agent: show inline error (D-02), do not send
```

### Advisory Reply Message Format

Messages table `metadata` column (JSONB, already exists) carries the advisory flag:

```json
{
  "advisory": true,
  "advisory_for_subtask": "subtask-uuid"
}
```

The frontend checks `message.metadata?.advisory === true` to render the visual distinction.

### API Response Type for Advisory Errors

The `POST /tasks/{id}/messages` endpoint returns two possible shapes:
- **Happy path** (no advisory errors): `Message` object directly
- **Advisory errors present**: `{ message: Message, advisory_errors: string[] }`

The frontend uses a discriminated union type (`Message | SendMessageResponse`) with an `isSendMessageResponse` type guard to safely handle both cases without `as` casts.

### Recommended Project Structure Changes

```
internal/
  executor/
    executor.go           # Add SendAdvisory method (no ctx param)
  handlers/
    messages.go           # Refactor routeToAgents for advisory routing
web/
  components/
    chat/
      GroupChat.tsx        # Extend autocomplete, add typing indicator
      ChatMessage.tsx      # Add advisory reply label
      TypingIndicator.tsx  # NEW: reusable typing dots component
  lib/
    store.ts              # Add typingAgents state to TaskStore
    api.ts                # Add SendMessageResponse type + isSendMessageResponse guard
    types.ts              # No changes needed (metadata already optional Record)
```

### Anti-Patterns to Avoid
- **Modifying SendFollowUp:** Never add "advisory mode" flag to SendFollowUp. That method has correct behavior for input_required resolution. Separate methods with clear responsibilities.
- **Shared A2A context:** Advisory must use `context.Background()` with its own timeout, never inherit from the executor's task context (D-16). If the task completes while advisory is in-flight, the advisory must still finish gracefully.
- **Passing ctx to SendAdvisory:** The method creates its own background context. Callers launch it via `go executor.SendAdvisory(taskID, ...)` without passing ctx.
- **Blocking the HTTP response on A2A call:** The advisory A2A call happens in a goroutine. The HTTP response to the user's message send returns immediately with the saved message. SSE delivers the response asynchronously.
- **Storing advisory state in subtasks table:** Advisory is a chat-layer feature. The subtasks table tracks DAG execution. Never add advisory columns to subtasks.
- **Using `as` casts for api response:** Use the `isSendMessageResponse` type guard instead of `as Record<string, unknown>` casts on the api.messages.send result.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Typing indicator animation | Custom CSS keyframes | Tailwind `animate-pulse` on dot elements | Consistent with AgentStatusDot working animation [VERIFIED: AgentStatusDot.tsx uses animate-pulse] |
| A2A message sending | New HTTP client | Existing `a2a.Client.SendMessage` | Already handles JSON-RPC 2.0 envelope, error parsing, response normalization [VERIFIED: a2a/client.go] |
| SSE event publishing | Direct WebSocket | Existing `Broker.Publish` for transient events | Typing indicators are ephemeral — use Broker-only, not EventStore+Broker [VERIFIED: events/broker.go] |
| Message deduplication | Custom dedup logic | Existing store pattern (`s.messages.some(m => m.id === msgId)`) | Already handles optimistic updates vs SSE delivery [VERIFIED: store.ts:280] |
| @mention parsing | New regex | Existing `mentionRe` (`<@([^|]+)\|[^>]+>`) | Already tested and working in both handlers [VERIFIED: messages.go:32, conversations.go:317] |

**Key insight:** This phase is primarily about wiring existing infrastructure together with correct isolation semantics. The A2A client, SSE broker, mention parsing, and autocomplete UI all exist. The new work is the advisory-specific routing logic and the typing indicator.

## Common Pitfalls

### Pitfall 1: Advisory Modifying Subtask Status
**What goes wrong:** Using `SendFollowUp` directly causes the advisory response to mark the subtask as "completed", which breaks the DAG executor. The subtask was still running its real task.
**Why it happens:** D-05 says "route via existing SendFollowUp" which could be interpreted literally.
**How to avoid:** Create a new `SendAdvisory` method that reuses A2A client but never writes to the subtasks table. Only writes to the messages table.
**Warning signs:** After an advisory reply, the DAG node shows as "completed" prematurely, and the agent's real work is lost.

### Pitfall 2: Context Cancellation Propagation
**What goes wrong:** Advisory goroutine inherits the executor's task context. When the task completes or is cancelled, the advisory context is also cancelled, causing the A2A call to fail mid-flight. The in-flight A2A agent may be left in an inconsistent state.
**Why it happens:** Using `ctx` from the executor instead of `context.Background()`.
**How to avoid:** Per D-16, always create a fresh `context.WithTimeout(context.Background(), 60*time.Second)` for advisory calls. Never pass the executor's context. SendAdvisory takes NO ctx parameter — it creates its own internally.
**Warning signs:** "context canceled" errors in advisory goroutine logs when tasks complete quickly.

### Pitfall 3: Race Between Advisory and Subtask Completion
**What goes wrong:** User sends advisory to Agent X while Agent X's subtask is "running". Between the advisory validation check and the A2A call, the subtask completes. Now the advisory is sending to an agent that finished.
**Why it happens:** Time-of-check-to-time-of-use (TOCTOU) race on subtask status.
**How to avoid:** This is acceptable by design. The advisory is fire-and-forget. If the subtask completed in the meantime, the A2A agent either responds (advisory reply appears after completion) or returns an error (timeout message appears). Neither crashes the system. Document this as expected behavior.
**Warning signs:** Advisory replies appearing after "Agent X completed the task" system message. This is fine -- not a bug.

### Pitfall 4: Concurrent Advisory Flooding
**What goes wrong:** User rapidly sends many @mentions to the same agent. Multiple goroutines make concurrent A2A calls, potentially overwhelming the agent.
**Why it happens:** No rate limiting or concurrency cap on advisory calls.
**How to avoid:** Consider capping concurrent advisories per agent per task (e.g., 1 at a time, queue subsequent). At minimum, the 60s timeout prevents unbounded accumulation.
**Warning signs:** Multiple "Agent X is typing..." indicators simultaneously, agent returning 429/errors.

### Pitfall 5: Typing Indicator Stuck
**What goes wrong:** "Agent X is typing..." never goes away because the goroutine panicked or the SSE event was dropped.
**Why it happens:** If the advisory goroutine exits without publishing "agent.typing_stopped", the frontend has no signal to clear the indicator.
**How to avoid:** Use `defer` in the advisory goroutine to always publish "agent.typing_stopped", regardless of success/failure/panic. Also add a client-side timeout (65s, slightly longer than backend 60s) that auto-clears the indicator.
**Warning signs:** Typing indicator persisting indefinitely after the advisory should have resolved.

### Pitfall 6: D-02 Validation Returning Error vs Inline Error
**What goes wrong:** When user @mentions a non-active agent, the backend returns HTTP 400 and the message is not saved. The user's message disappears.
**Why it happens:** Validation happens at the wrong layer -- rejecting the HTTP request instead of saving the message and showing an inline error.
**How to avoid:** The message should always be saved (it's a valid user message). The validation error should be returned as additional data in the response, and/or published as a system message in the chat. The frontend can show an inline error toast while still keeping the user's message visible.
**Warning signs:** User's message with @mention to inactive agent vanishes after hitting send.

### Pitfall 7: Empty Autocomplete When Subtasks Not Loaded
**What goes wrong:** The @mention autocomplete dropdown is empty when subtasks have not loaded yet (subtasks prop is undefined).
**Why it happens:** Filtering to `hasSubtask === true` when `subtasks` is undefined means `subtasks.filter(...)` returns `[]` for every agent, so `hasSubtask` is always false.
**How to avoid:** When `subtasks` is undefined or empty, fall back to showing all agents without status filtering (`isActive: true, hasSubtask: true`). Only apply the hasSubtask filter when subtasks data is defined and non-empty.
**Warning signs:** Autocomplete works on second page load but not on first, or appears empty until a subtask SSE event arrives.

## Code Examples

### Backend: SendAdvisory Method (Executor)

```go
// Source: New method based on codebase patterns in executor.go
// SendAdvisory sends an advisory message to an agent without modifying subtask status.
// Used for @mention routing when a user sends an advisory message during task execution.
// NOTE: No ctx parameter — creates its own background context per D-16.
func (e *DAGExecutor) SendAdvisory(taskID, subtaskID, agentID, content string) {
    // Create isolated context with 60s timeout (D-14, D-16)
    advCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    // Publish typing indicator (transient — Broker-only, not EventStore)
    e.publishTransientEvent(taskID, "agent.typing", map[string]any{
        "agent_id":   agentID,
        "agent_name": agentName, // resolved from DB
    })

    // Always clear typing indicator on exit
    defer func() {
        e.publishTransientEvent(taskID, "agent.typing_stopped", map[string]any{
            "agent_id": agentID,
        })
    }()

    // Build advisory context (D-04): user message + subtask description + previous output
    var instruction string
    var prevOutput json.RawMessage
    _ = e.DB.QueryRow(advCtx,
        `SELECT instruction, COALESCE(output, 'null') FROM subtasks WHERE id = $1`,
        subtaskID).Scan(&instruction, &prevOutput)

    advisoryContent := fmt.Sprintf(
        "Advisory message from user: %s\n\nYour current task: %s",
        content, instruction)
    if len(prevOutput) > 0 && string(prevOutput) != "null" {
        // Truncate if very long
        outputStr := string(prevOutput)
        if len(outputStr) > 2000 {
            outputStr = outputStr[:2000] + "... (truncated)"
        }
        advisoryContent += fmt.Sprintf("\n\nYour previous output: %s", outputStr)
    }

    // Load A2A task ID and endpoint
    var a2aTaskID, agentEndpoint string
    err := e.DB.QueryRow(advCtx,
        `SELECT s.a2a_task_id, a.endpoint FROM subtasks s JOIN agents a ON a.id = s.agent_id WHERE s.id = $1`,
        subtaskID).Scan(&a2aTaskID, &agentEndpoint)
    if err != nil {
        log.Printf("advisory: load subtask %s: %v", subtaskID, err)
        e.publishAdvisoryError(advCtx, taskID, agentID, agentName, "Could not reach the agent")
        return
    }

    parts := []a2a.MessagePart{a2a.TextPart(advisoryContent)}

    // Send A2A message -- use taskID as contextID for advisory (separate from executor)
    result, err := e.A2AClient.SendMessage(advCtx, agentEndpoint, taskID, a2aTaskID, parts)
    if err != nil {
        // Retry once (D-14)
        result, err = e.A2AClient.SendMessage(advCtx, agentEndpoint, taskID, a2aTaskID, parts)
        if err != nil {
            e.publishAdvisoryError(advCtx, taskID, agentID, agentName, "did not respond to the advisory message")
            return
        }
    }

    // Poll if async
    if result.State == "working" || result.State == "submitted" {
        result = e.pollUntilTerminal(advCtx, agentEndpoint, result)
    }

    // Publish advisory response as chat message (NOT subtask status change)
    if result.State == "completed" && len(result.Artifacts) > 0 {
        // Save message with advisory metadata
        e.publishAdvisoryMessage(advCtx, taskID, agentID, agentName, result.Artifacts, subtaskID)
    } else if result.State == "failed" {
        e.publishAdvisoryError(advCtx, taskID, agentID, agentName, "encountered an error processing the advisory")
    }
}
```

### Backend: Advisory Message with Metadata

```go
// Source: Pattern from executor.go publishMessage, extended with metadata
func (e *DAGExecutor) publishAdvisoryMessage(ctx context.Context, taskID, senderID, senderName string, artifacts json.RawMessage, subtaskID string) {
    msgID := uuid.New().String()
    now := time.Now()

    var conversationID string
    _ = e.DB.QueryRow(ctx, `SELECT COALESCE(conversation_id, '') FROM tasks WHERE id = $1`, taskID).Scan(&conversationID)

    metadata, _ := json.Marshal(map[string]any{
        "advisory":             true,
        "advisory_for_subtask": subtaskID,
    })

    _, err := e.DB.Exec(ctx,
        `INSERT INTO messages (id, task_id, conversation_id, sender_type, sender_id, sender_name, content, mentions, metadata, created_at)
         VALUES ($1, $2, $3, 'agent', $4, $5, $6, '{}', $7, $8)`,
        msgID, taskID, conversationID, senderID, senderName, content, metadata, now)
    if err != nil {
        log.Printf("executor: insert advisory message: %v", err)
        return
    }

    e.publishEvent(ctx, taskID, "", "message", "agent", senderID, map[string]any{
        "message_id":  msgID,
        "sender_name": senderName,
        "sender_type": "agent",
        "content":     content,
        "metadata":    map[string]any{"advisory": true, "advisory_for_subtask": subtaskID},
    })
}
```

### Backend: Validation in routeAdvisory

```go
// Source: Refactor of messages.go:155 routeToAgents
// Returns list of validation errors for non-active agents (D-02)
func (h *MessageHandler) routeAdvisory(ctx context.Context, taskID string, mentions []string, content string) []string {
    var errors []string

    for _, agentID := range mentions {
        var subtaskID, agentName string
        err := h.DB.QueryRow(ctx,
            `SELECT s.id, a.name
             FROM subtasks s JOIN agents a ON a.id = s.agent_id
             WHERE s.task_id = $1 AND s.agent_id = $2 AND s.status IN ('running', 'input_required')
             ORDER BY s.created_at DESC
             LIMIT 1`, taskID, agentID).
            Scan(&subtaskID, &agentName)
        if err != nil {
            // Agent not active -- collect error (D-02)
            var name string
            _ = h.DB.QueryRow(ctx, `SELECT name FROM agents WHERE id = $1`, agentID).Scan(&name)
            if name == "" {
                name = "Agent"
            }
            errors = append(errors, fmt.Sprintf("%s is not currently executing", name))
            continue
        }

        // Route via advisory (D-05 pattern: detached goroutine, SendAdvisory creates own background context)
        go h.Executor.SendAdvisory(taskID, subtaskID, agentID, content)
    }

    return errors
}
```

### Frontend: Advisory Reply Label in ChatMessage

```tsx
// Source: Extension of ChatMessage.tsx:274
// Check metadata for advisory flag
const isAdvisory = useMemo(() => {
  if (!message.metadata) return false;
  return (message.metadata as Record<string, unknown>).advisory === true;
}, [message.metadata]);

// In the render:
<div className="flex items-baseline gap-2">
  <span className="text-sm font-medium text-foreground">
    {message.sender_name}
  </span>
  {isAdvisory && (
    <span className="rounded-full bg-blue-500/10 px-2 py-0.5 text-[10px] font-medium text-blue-400">
      Advisory reply
    </span>
  )}
  <span className="text-[10px] text-muted-foreground">
    {formatTime(message.created_at)}
  </span>
</div>
```

### Frontend: Typing Indicator Component

```tsx
// Source: New component based on project Tailwind patterns
interface TypingIndicatorProps {
  agentName: string;
}

export function TypingIndicator({ agentName }: TypingIndicatorProps) {
  return (
    <div className="flex items-center gap-2 px-4 py-2">
      <div className="flex gap-1">
        <span className="size-1.5 animate-bounce rounded-full bg-blue-400 [animation-delay:0ms]" />
        <span className="size-1.5 animate-bounce rounded-full bg-blue-400 [animation-delay:150ms]" />
        <span className="size-1.5 animate-bounce rounded-full bg-blue-400 [animation-delay:300ms]" />
      </div>
      <span className="text-xs text-muted-foreground">
        {agentName} is typing...
      </span>
    </div>
  );
}
```

### Frontend: Autocomplete with Status (GroupChat.tsx)

```tsx
// Source: Extension of GroupChat.tsx filteredAgents
// Derive agent status from subtasks — handle undefined subtasks gracefully
const agentsWithStatus = useMemo(() => {
  return agents.map((agent) => {
    if (!subtasks || subtasks.length === 0) {
      // Subtasks not loaded yet — show all agents, assume potentially active
      return { ...agent, isActive: true, hasSubtask: true };
    }
    const agentSubtasks = subtasks.filter((st) => st.agent_id === agent.id);
    const hasSubtask = agentSubtasks.length > 0;
    const isActive = agentSubtasks.some(
      (st) => st.status === "running" || st.status === "input_required"
    );
    return { ...agent, isActive, hasSubtask };
  });
}, [agents, subtasks]);

// In autocomplete dropdown:
<button
  className={cn(
    "flex w-full items-center gap-2 px-3 py-2 text-left text-sm",
    !agent.isActive && "opacity-50"
  )}
>
  <AgentStatusDot status={agent.isActive ? "working" : "idle"} />
  <span>{agent.name}</span>
  {!agent.isActive && (
    <span className="text-[10px] text-muted-foreground">(not active)</span>
  )}
</button>
```

### Frontend: API Type Guard for Advisory Errors

```typescript
// In web/lib/api.ts
export interface SendMessageResponse {
  message: Message;
  advisory_errors: string[];
}

export function isSendMessageResponse(
  result: Message | SendMessageResponse
): result is SendMessageResponse {
  return "advisory_errors" in result && "message" in result;
}

// In store.ts — use type guard instead of unsafe casts:
const result = await api.messages.send(taskId, content);
const msg = isSendMessageResponse(result) ? result.message : result;
const advisoryErrors = isSendMessageResponse(result) ? result.advisory_errors : undefined;
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `routeToAgents` calls `SendFollowUp` directly | New `SendAdvisory` method for advisory, `SendFollowUp` unchanged for input_required | Phase 5 | Advisory messages no longer modify subtask status |
| Autocomplete shows all agents equally | Autocomplete shows agent activity status with AgentStatusDot | Phase 5 | Users can see which agents are available for advisory |
| No typing indicators | SSE-driven "Agent X is typing..." with auto-clear | Phase 5 | Users get feedback that advisory message was received |
| api.messages.send returns `Message` only | Returns `Message | SendMessageResponse` with type guard | Phase 5 | Frontend handles advisory errors without unsafe casts |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | A2A agents will respond to advisory messages on existing taskID/contextID without requiring a new task creation | Architecture Patterns | Need to create new A2A tasks instead of reusing existing ones -- moderate refactor |
| A2 | The `metadata` JSONB column in messages table accepts arbitrary JSON including `{"advisory": true}` | Architecture Patterns | Would need a migration to add an advisory-specific column -- minor |
| A3 | Existing `pollUntilTerminal` with advisory's 60s context timeout will correctly short-circuit when context expires | Code Examples | May need separate polling logic with shorter intervals for advisory |
| A4 | Typing indicator SSE events (`agent.typing`, `agent.typing_stopped`) are transient and do not need persistence to the events table | Architecture Patterns | If persisted, they would clutter the event replay on page reconnect |

**A2 Verification:** The `messages` table has `metadata JSONB NOT NULL DEFAULT '{}'` (migration 001_foundation.sql:156). The `publishMessage` helper already writes to the metadata column. [VERIFIED: migration file] -- this assumption is confirmed, promoting to VERIFIED.

**A4 Resolution:** Typing indicators are ephemeral UI state. They are published via `Broker.Publish()` only (not `EventStore.Save()`) to avoid polluting the event replay. If a user reconnects during a typing indicator, they will simply not see it -- the response message will appear shortly after anyway. The `publishTransientEvent` method on DAGExecutor implements this Broker-only pattern. [RESOLVED: confirmed in plan, Broker-only approach adopted]

## Open Questions

1. **A2A contextID for advisory messages** (RESOLVED)
   - What we know: `SendFollowUp` uses the task's existing `a2a_task_id` as the A2A taskID parameter. Advisory should NOT share this context to avoid polluting the main conversation.
   - Resolution: Use the taskID as contextID but pass the existing `a2a_task_id` as the A2A task parameter. This gives the agent conversation grouping context without creating a brand new A2A task. Adopted in Plan 01 SendAdvisory implementation.

2. **Subtask context truncation** (RESOLVED)
   - What we know: D-04 says to include "subtask's previous output" in the advisory context.
   - Resolution: Truncate previous output at 2000 characters with "... (truncated)" suffix. This is Claude's Discretion per CONTEXT.md. Adopted in Plan 01 SendAdvisory implementation.

3. **Concurrent advisory cap** (RESOLVED — deferred)
   - What we know: Multiple rapid @mentions could spawn many goroutines.
   - Resolution: No cap in Phase 5. The 60s context timeout prevents unbounded goroutine accumulation. Capping is Claude's Discretion per CONTEXT.md and can be added later if agents show issues with concurrent advisories. The current approach is simpler and adequate for the advisory-only use case.

4. **SendAdvisory ctx parameter** (RESOLVED)
   - What we know: D-16 says advisory must use separate context, not executor's.
   - Resolution: SendAdvisory takes NO ctx parameter. It creates `context.WithTimeout(context.Background(), 60s)` internally. Callers invoke via `go executor.SendAdvisory(taskID, subtaskID, agentID, content)`. This makes D-16 isolation impossible to violate at the call site.

5. **api.ts return type for advisory errors** (RESOLVED)
   - What we know: Backend returns `Message` normally, `{ message: Message, advisory_errors: string[] }` when advisory errors exist.
   - Resolution: api.ts exports `SendMessageResponse` interface and `isSendMessageResponse` type guard. `messages.send` returns `Promise<Message | SendMessageResponse>`. Store uses type guard instead of `as` casts.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Existing `RequireAuth` middleware on message endpoints [VERIFIED: cmd/server/main.go route registration] |
| V3 Session Management | no | No new sessions introduced |
| V4 Access Control | yes | User can only send advisories to agents in tasks they own (existing task ownership check) |
| V5 Input Validation | yes | Message content already validated (non-empty, trimmed); @mention format validated by regex |
| V6 Cryptography | no | No new crypto operations |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| User sends advisory to agent in another user's task | Elevation of Privilege | Verify task ownership before routing advisory (existing auth middleware) |
| Malformed @mention injection | Tampering | `mentionRe` regex already validates format; agent ID validated against DB |
| Advisory content injection into A2A prompt | Information Disclosure | Advisory content is wrapped in a clear "Advisory message from user:" prefix; agent system prompt handles separation |
| Goroutine leak from stuck advisory | Denial of Service | 60s context timeout ensures goroutines always terminate; defer cleanup |

## Sources

### Primary (HIGH confidence)
- `internal/handlers/messages.go` -- Full MessageHandler implementation including routeToAgents [VERIFIED: codebase read]
- `internal/executor/executor.go` -- DAGExecutor.SendFollowUp implementation, subtask status modification behavior [VERIFIED: codebase read]
- `internal/events/broker.go` -- Broker pub/sub with task and conversation routing [VERIFIED: codebase read]
- `internal/events/store.go` -- EventStore persistence and replay [VERIFIED: codebase read]
- `internal/a2a/client.go` -- A2A JSON-RPC client with SendMessage, GetTask, CancelTask [VERIFIED: codebase read]
- `web/components/chat/GroupChat.tsx` -- Autocomplete state, mention handling [VERIFIED: codebase read]
- `web/components/chat/ChatMessage.tsx` -- Message rendering with sender attribution [VERIFIED: codebase read]
- `web/components/agent/AgentStatusDot.tsx` -- Reusable status indicator [VERIFIED: codebase read]
- `web/lib/store.ts` -- Zustand stores with SSE event handling [VERIFIED: codebase read]
- `web/lib/sse.ts` -- SSE connection helpers [VERIFIED: codebase read]
- `web/lib/types.ts` -- TypeScript interfaces matching Go models [VERIFIED: codebase read]
- `web/lib/api.ts` -- API client with typed endpoints [VERIFIED: codebase read]
- `internal/db/migrations/001_foundation.sql` -- Messages table schema with metadata JSONB column [VERIFIED: codebase read]

### Secondary (MEDIUM confidence)
- `.planning/phases/05-chat-intervention/05-CONTEXT.md` -- All locked decisions (D-01 through D-16) [VERIFIED: file read]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries already in use, no new dependencies
- Architecture: HIGH -- based on thorough reading of existing codebase patterns, all extension points identified
- Pitfalls: HIGH -- derived from actual code analysis (SendFollowUp status mutation, context propagation, TOCTOU race, undefined subtasks)

**Research date:** 2026-04-04
**Valid until:** 2026-05-04 (stable -- all patterns based on existing codebase)
