# Phase 5: Chat Intervention - Context

**Gathered:** 2026-04-05
**Status:** Ready for planning

<domain>
## Phase Boundary

User can send advisory messages to specific sub-agents during active task execution via @mention in the chat input. Agent responses appear inline in the conversation stream with correct sender attribution. Advisory-only mode — no directive control over agent behavior.

</domain>

<decisions>
## Implementation Decisions

### Message Routing Strategy
- **D-01:** Advisory-only model — user messages are informational, agents may or may not act on them (directive mode deferred to v2)
- **D-02:** When user @mentions an agent with no active (running/input_required) subtask, block the send and show inline error: "Agent X is not currently executing — advisory messages can only be sent to active agents"
- **D-03:** Multiple @mentions in a single message route independently — each mentioned agent receives the full message and responds independently, responses appear in conversation flow as they arrive
- **D-04:** Advisory message context sent to A2A agent: user message text + current subtask description + subtask's previous output (so agent understands what it's working on)
- **D-05:** Messages routed via existing `SendFollowUp` mechanism in a detached goroutine with background context

### Conversation Presentation
- **D-06:** Agent advisory replies use existing ChatMessage component but with a subtle visual distinction — small "Advisory reply" label or different background tint to differentiate from normal agent execution messages
- **D-07:** Show "Agent X is typing..." indicator in the conversation flow after user sends an advisory @mention, until the response arrives or timeout
- **D-08:** User's advisory messages display like normal user messages with @mention highlighting — no special advisory tag on the user side
- **D-09:** Typing indicator implemented via SSE event (`agent.typing` on the task's channel) — published when follow-up starts, cleared when response arrives or times out

### @Mention Interaction Design
- **D-10:** @mention autocomplete list shows all agents that have participated in the current task (any agent with at least one subtask), with status indicators showing which are currently active (running/input_required) vs completed/pending
- **D-11:** Agents not currently active appear grayed out in the autocomplete with "(not active)" label — selecting them triggers the D-02 block behavior
- **D-12:** After sending, user message appears immediately in conversation flow, then typing indicator starts — clear feedback loop
- **D-13:** Existing `showMentions` and `mentionFilter` state in GroupChat.tsx extended (not replaced) with status-aware filtering

### Executor Safety
- **D-14:** Advisory A2A calls run with 60-second timeout, automatic retry once on failure, then show "Agent X did not respond" error message in conversation flow
- **D-15:** Complete isolation — advisory calls run in independent goroutines, failures only produce error messages in conversation, executor continues unaffected regardless of advisory outcome
- **D-16:** Advisory follow-up uses a separate A2A context (not the executor's main context) to prevent cancellation propagation — if the task completes or is canceled while an advisory is in-flight, the advisory goroutine still completes gracefully

### Claude's Discretion
- Exact "Advisory reply" label text and styling (color, border, background)
- Typing indicator animation implementation (dots, spinner, etc.)
- Whether to cap the number of concurrent advisory calls per task
- Error message wording for timeout/failure scenarios
- Subtask context truncation strategy if previous output is very long

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Chat & Messaging Infrastructure
- `internal/handlers/messages.go` — MessageHandler with Send, List, routeToAgents, mentionRe regex
- `internal/handlers/conversations.go` — ConversationHandler with mention extraction pattern
- `internal/executor/executor.go` — SendFollowUp method, input_required state handling, DAG execution loop
- `internal/models/event.go` — Event model with Mentions field

### A2A Protocol
- `internal/a2a/client.go` — A2A client for agent communication
- `internal/a2a/aggregator.go` — Conversation context aggregation (contextId → history)
- `internal/a2a/protocol.go` — A2A message envelope (role + text/artifact parts)

### Frontend Chat Components
- `web/components/chat/GroupChat.tsx` — Chat UI with showMentions autocomplete, hasWaitingSubtask detection
- `web/components/chat/ChatMessage.tsx` — Message rendering with sender attribution
- `web/components/conversation/ConversationView.tsx` — Conversation container

### SSE Infrastructure
- `internal/events/broker.go` — Broker with Subscribe/Publish, task_id keying
- `internal/handlers/stream.go` — SSE handlers (per-task + multiplexed)
- `web/lib/sse.ts` — connectSSE, connectConversationSSE, connectMultiTaskSSE helpers
- `web/lib/store.ts` — Zustand stores with SSE event handling

### Agent Status (Phase 2)
- `web/components/agent/AgentStatusDot.tsx` — Reusable status indicator for @mention list
- `web/lib/store.ts` — useAgentStore.agentStatuses for real-time agent status

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `MessageHandler.routeToAgents()` — already parses @mentions and calls SendFollowUp in detached goroutine; needs extension for timeout/retry and typing indicator
- `mentionRe` regex — `<@([^|]+)\|[^>]+>` already extracts agent IDs from mention format
- `GroupChat.tsx` showMentions/mentionFilter/mentionIndex — autocomplete state exists, needs agent status enrichment
- `hasWaitingSubtask` — already detects input_required subtasks in GroupChat
- `ChatMessage.tsx` — message rendering with avatar, sender name, timestamp; extend with advisory label
- `AgentStatusDot` — reusable for @mention autocomplete list status indicators

### Established Patterns
- Detached goroutine with `context.Background()` for A2A calls (already in routeToAgents)
- SSE event publishing via `EventStore.Save()` + `Broker.Publish()` — use same pattern for typing indicator
- Message model includes `mentions []string` and `metadata json.RawMessage` — metadata can carry advisory flag
- Subscribe-before-replay SSE pattern from Phase 2

### Integration Points
- `messages.go:routeToAgents` — primary extension point for timeout/retry/typing logic
- `GroupChat.tsx` — extend autocomplete with agent status, add typing indicator
- `ChatMessage.tsx` — add advisory reply visual distinction
- `web/lib/store.ts` — handle new SSE events (agent.typing, advisory response)
- `cmd/server/main.go` — no new routes needed; existing POST /tasks/{id}/messages handles it

</code_context>

<specifics>
## Specific Ideas

- User wants to be able to invite agents not in the current task into the conversation — deferred to future phase as it requires executor DAG modification
- Advisory replies should feel natural in the conversation flow, not like a separate notification system

</specifics>

<deferred>
## Deferred Ideas

- **Dynamic agent invitation:** User wants to @mention agents not participating in the current task and have them join. This requires modifying the executor's DAG to dynamically create subtasks — a significant architectural change. Belongs in a future phase.
- **Directive mode:** Sending commands that directly affect agent behavior (e.g., "stop", "change approach") — deferred to v2 per STATE.md decision.
- **Multi-turn advisory conversations:** Threaded back-and-forth between user and agent within an advisory context — Phase 5 is single-round advisory only.

</deferred>

---

*Phase: 05-chat-intervention*
*Context gathered: 2026-04-05*
