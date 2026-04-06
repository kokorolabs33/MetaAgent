# Phase 5: Chat Intervention - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-05
**Phase:** 05-chat-intervention
**Areas discussed:** Message Routing Strategy, Conversation Presentation, @Mention Interaction Design, Executor Safety

---

## Message Routing Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Show hint but still send | Send message with warning that agent isn't active | |
| Block send (recommended) | Don't allow @mention of non-active agents, show inline error | ✓ |
| Queue for later | Store message and deliver when agent's subtask starts | |

**User's choice:** Block send and show prompt
**Notes:** User initially asked "what does 'no active subtask' mean?" — explained the concept of running/input_required vs completed/pending subtask states. After understanding, chose to block.

| Option | Description | Selected |
|--------|-------------|----------|
| Route independently (recommended) | Each mentioned agent receives full message and responds independently | ✓ |
| Route to first only | Only send to first @mentioned agent | |
| Merge as single message | All agents receive same message, only one responds | |

**User's choice:** Route independently

| Option | Description | Selected |
|--------|-------------|----------|
| User message + recent N conversation history | Send user message plus last 5-10 channel messages | |
| User message only | Only send raw user message text (current behavior) | |
| User message + subtask context | Send user message + agent's current subtask description + previous output | ✓ |

**User's choice:** User message + subtask context

## Conversation Presentation

| Option | Description | Selected |
|--------|-------------|----------|
| Same style as normal messages | Use existing ChatMessage, differentiate only by sender | |
| Slightly different style (recommended) | Keep agent name/avatar, add "Advisory reply" label or different tint | ✓ |
| Distinctly different style | Dedicated advisory message card with quote of user's original message | |

**User's choice:** Slightly different style

| Option | Description | Selected |
|--------|-------------|----------|
| Show typing indicator (recommended) | "Agent X is typing..." animation after send, until response or timeout | ✓ |
| Don't show | No intermediate feedback, reply just appears | |

**User's choice:** Show typing indicator

| Option | Description | Selected |
|--------|-------------|----------|
| Display as normal (recommended) | Same as regular user messages with @mention highlighting | ✓ |
| Add advisory tag | Show "Advisory" label next to user message | |

**User's choice:** Display as normal

## @Mention Interaction Design

| Option | Description | Selected |
|--------|-------------|----------|
| Only active agents (recommended) | Show only agents with running/input_required subtasks | |
| All participating agents | Show all agents that have had subtasks, with status markers | ✓ (modified) |
| All registered agents | Show every agent in the system | |

**User's choice:** Show all participating agents with status indicators
**Notes:** User raised the idea of inviting agents not in the current task. This was captured as a deferred idea (dynamic agent invitation requires executor DAG modification). Settled on showing all participating agents with active/inactive status indicators.

| Option | Description | Selected |
|--------|-------------|----------|
| Message + typing indicator (recommended) | User message appears immediately, then typing indicator shows | ✓ |
| Message only | Just show the sent message, no additional feedback | |
| Toast notification | Show "Message sent to Agent X" toast | |

**User's choice:** Message + typing indicator

## Executor Safety

| Option | Description | Selected |
|--------|-------------|----------|
| 30-second timeout (recommended) | 30s timeout, no retry, show error on timeout | |
| 60-second + auto retry | 60s timeout, retry once, then show error | ✓ |
| Claude decides | Let technical research determine appropriate timeout/retry | |

**User's choice:** 60-second timeout with automatic retry

| Option | Description | Selected |
|--------|-------------|----------|
| Complete isolation (recommended) | Advisory runs in independent goroutine, failures only produce error messages | ✓ |
| Partial isolation | Notify executor of advisory failure, executor may adjust strategy | |

**User's choice:** Complete isolation

## Claude's Discretion

- Exact "Advisory reply" label text and visual styling
- Typing indicator animation implementation
- Concurrent advisory call cap per task
- Error message wording
- Subtask context truncation strategy

## Deferred Ideas

- Dynamic agent invitation: @mention agents not in current task, have them join (requires DAG modification)
- Directive mode: Commands that control agent behavior (v2)
- Multi-turn advisory conversations: Threaded back-and-forth within advisory context
