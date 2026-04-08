# Chat-First UI Redesign

**Date:** 2026-03-28
**Status:** Draft
**Author:** Jasper + Claude

## Overview

Redesign the TaskHub frontend from a task management dashboard into a ChatGPT + Slack hybrid — a chat-first AI collaboration platform. The primary interface becomes a conversation where users interact with the orchestrator LLM and observe agents collaborating in a group chat.

### Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Primary interface | Chat, not dashboard | Users spend 90% of time watching conversations |
| Layout model | ChatGPT sidebar + Slack group chat + DAG panel | Familiar, proven patterns |
| Management access | Gear icon → separate panel | Management is setup-once; chat is daily use |
| Conversation:Task | 1:N | One conversation can spawn multiple tasks |
| DAG display | Wave-grouped right panel (fixed, scrollable) | Compact, handles parallel execution |
| Participant display | Stacked avatars in top bar, click to expand | Non-intrusive, always visible |
| Message organization | Flat in chat, optional task_id marker | Natural reading flow, DAG can highlight related messages |
| Multi-task DAG | Show active task, dropdown to switch | Clean, no clutter |

---

## 1. Data Model Changes

### New table: `conversations`

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | PK (UUID) |
| `title` | TEXT | User-editable title (default: first message summary) |
| `created_by` | TEXT | FK → users |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | Updated on each new message |

### Modified: `tasks`

| New Column | Type | Description |
|------------|------|-------------|
| `conversation_id` | TEXT | FK → conversations (required) |

A conversation can have 0-N tasks. Tasks are created when the orchestrator decides to decompose a user message into a DAG.

### Modified: `messages`

| Change | Description |
|--------|-------------|
| Replace `task_id` with `conversation_id` | Messages belong to conversations, not tasks |
| Add `task_id` (nullable) | Optional marker for which task a message relates to |

The `conversation_id` is the primary association. The `task_id` is metadata for UI highlighting when viewing a specific task's DAG.

### Modified: `events`

| Change | Description |
|--------|-------------|
| Add `conversation_id` | SSE streams subscribe by conversation, not task |

Events still have `task_id` for task-specific events (subtask status changes), but the SSE subscription is per-conversation so the frontend gets all events for all tasks in one stream.

### Migration summary

```sql
CREATE TABLE IF NOT EXISTS conversations (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS conversation_id TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN IF NOT EXISTS conversation_id TEXT NOT NULL DEFAULT '';
ALTER TABLE events ADD COLUMN IF NOT EXISTS conversation_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_tasks_conversation ON tasks(conversation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_events_conversation ON events(conversation_id, created_at);
```

---

## 2. Conversation Lifecycle

### Phase 1: New Conversation

User clicks "+ New Conversation" or starts typing in an empty state.

```
User sends first message
  → Create Conversation record (title = first ~50 chars of message)
  → Save message to conversation
  → Send to Orchestrator LLM with context:
      system prompt + user message + available agents list
  → Orchestrator decides: chat response OR task decomposition
```

### Phase 2: Intent Detection

The orchestrator LLM judges each user message:

- **Chat/clarification** → respond directly as "System", no task created
- **Actionable task** → create Task, decompose into DAG, execute

The intent detection is part of the orchestrator's system prompt — it returns a structured response indicating whether to create a task or just reply.

```
Orchestrator prompt includes:
  "If the user's message is a task that requires agent collaboration,
   respond with a task decomposition. If it's a question, clarification,
   or discussion, respond conversationally."
```

### Phase 3: Task Execution (Agents Join)

When a task is created:

```
1. System message: "Task created: [summary]. Decomposing..."
2. Orchestrator calls LLM to generate DAG
3. System message: "[Agent Name] joined the conversation" (for each agent)
4. DAG panel populates with Wave groups
5. Participant avatars update in top bar
6. Agents execute subtasks, results posted as chat messages
7. Each agent message tagged with task_id for DAG panel correlation
```

### Phase 4: Completion + Continuation

```
All subtasks complete
  → System message: "Task completed. [summary]"
  → DAG panel shows all green
  → User can continue chatting:
    - Ask follow-up questions (Orchestrator replies directly)
    - Request new work (creates Task 2, new DAG)
```

### Phase 5: Multi-Task in Same Conversation

When user requests new work after Task 1 completes:

```
User: "基于这个评估，写一份董事会报告"
  → Orchestrator sees full conversation history (including Task 1 results)
  → Creates Task 2 with conversation_id = same conversation
  → DAG panel switches to Task 2
  → Task 1 available via dropdown in DAG panel header
  → Task 2's subtask instructions include relevant Task 1 outputs
```

Context passing: the orchestrator's LLM call includes the full message history of the conversation. Since Task 1's agent results are in the messages, the LLM naturally has context about what was already done.

---

## 3. UI Layout

### Left Sidebar (w-64)

```
┌──────────────────┐
│ TaskHub logo      │
│ ──────────────── │
│ [+ New Chat]      │
│ ──────────────── │
│ Today             │
│   收购 Chatly 评估  │ ← active (highlighted)
│   东南亚市场策略     │
│ Yesterday         │
│   Q1 季度总结       │
│   竞品分析          │
│ ──────────────── │
│ ⚙️ Settings       │
│ 👤 User / Logout  │
└──────────────────┘
```

- Conversations sorted by `updated_at` DESC, grouped by date
- Each item shows: title (truncated), agent count, status indicator
- Click to switch conversation
- "+ New Chat" creates empty conversation state

### Top Bar

```
┌─────────────────────────────────────────────────────────┐
│ [Title ✏️]                                  [🟣🟡🟢🔵 5] │
└─────────────────────────────────────────────────────────┘
```

- **Left:** Editable conversation title. Click pencil to edit inline. Auto-generated from first message if not set.
- **Right:** Stacked participant avatars (overlapping circles). Shows user + all agents that have participated. Number badge. Click to expand into a dropdown list showing each participant with name, role (user/agent), and status (online/offline for agents).

### Chat Area (main)

Slack-style message feed:

```
[🤖] System · 2:30 PM
     Task decomposed into 5 subtasks across 4 departments.

[🟡] Marketing Dept · 2:31 PM
     ## AI Customer Service Market Report
     Global market size reached $12.3B in 2024...

[🟢] Legal Dept · 2:32 PM
     ## Compliance Risk Assessment
     IP ownership is clear, no pending litigation...
```

**Message types:**
- **User messages:** Purple avatar, right-aligned or left-aligned (Slack style = left)
- **Agent messages:** Unique color per agent, left-aligned, with markdown rendering
- **System messages:** Gray, italic, smaller font (task lifecycle notifications)
- **Join/leave notifications:** Inline, minimal ("Engineering Dept joined the conversation")

**Task correlation:** Messages with `task_id` set get a subtle colored left-border matching the task's color in the DAG panel. Only visible when user hovers or when a specific task is selected in the DAG dropdown.

### Input Area

```
┌────────────────────────────────────────────┐
│ Type a message... @ to mention an agent    │ [↑]
└────────────────────────────────────────────┘
```

- Auto-resize textarea
- @mention autocomplete for agents in current conversation
- Enter to send, Shift+Enter for newline
- Send button

### DAG Panel (right, w-48)

```
┌─────────────────────┐
│ Progress  [Task ▾] ↗ │
│ ─────────────────── │
│ WAVE 1 · Parallel    │
│ │ ● Marketing  ✓ 45s │
│ │ ● Legal      ✓ 52s │
│ │ ● Finance    ⟳ 30s │
│                      │
│ WAVE 2 · Waiting     │
│ │ ○ Engineering  ⏳   │
│                      │
│ WAVE 3 · Final       │
│ │ ○ Report       ⏳   │
│ ─────────────────── │
│ ████████░░ 2/5 · 40% │
└──────────────────────┘
```

- **Header:** "Progress" label + Task selector dropdown + "↗" expand button
- **Task dropdown:** Lists all tasks in conversation: "Task 1: 收购评估 ✓ / Task 2: 董事会报告 ⟳"
- **Wave groups:** Each wave shows parallel steps with status icons and duration
- **Click a step:** Opens the SubtaskDetailPanel (slide-out, already built)
- **Progress bar:** Overall completion percentage
- **Expand button (↗):** Opens full ReactFlow DAG in a modal/overlay
- **Empty state:** When no task exists yet: "Waiting for task..." or hidden
- **Fixed height, scrollable:** Panel doesn't grow — content scrolls within

### Empty Conversation State

When user opens "+ New Chat", the main area shows:

```
        ┌─────────────────────────┐
        │                         │
        │      TaskHub logo       │
        │                         │
        │  How can I help?        │
        │                         │
        │  ┌───────────────────┐  │
        │  │ Describe a task...│  │
        │  └───────────────────┘  │
        │                         │
        │  Quick suggestions:     │
        │  [评估收购方案]          │
        │  [竞品分析]             │
        │  [季度总结]             │
        └─────────────────────────┘
```

Centered input with optional quick-start suggestions. No DAG panel shown until a task is created.

---

## 4. Backend API Changes

### New endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/conversations` | List conversations (paginated, sorted by updated_at) |
| POST | `/api/conversations` | Create new conversation |
| GET | `/api/conversations/{id}` | Get conversation detail |
| PUT | `/api/conversations/{id}` | Update title |
| DELETE | `/api/conversations/{id}` | Delete conversation + cascade |
| GET | `/api/conversations/{id}/messages` | Get messages for conversation |
| POST | `/api/conversations/{id}/messages` | Send message (triggers orchestrator) |
| GET | `/api/conversations/{id}/tasks` | List tasks in conversation |
| GET | `/api/conversations/{id}/events` | SSE stream for conversation |

### Modified endpoints

| Endpoint | Change |
|----------|--------|
| `POST /api/tasks` | No longer the primary entry point. Tasks created internally by orchestrator |
| `GET /api/tasks/{id}/events` | Keep for backward compatibility, but primary stream is per-conversation |

### Message send flow

`POST /api/conversations/{id}/messages` does:

1. Save user message to DB
2. Publish SSE event
3. Call orchestrator with full conversation context
4. Orchestrator returns: `{type: "chat", content: "..."}` or `{type: "task", plan: {...}}`
5. If chat: save system message, publish SSE
6. If task: create Task, create subtasks, start execution, publish events

---

## 5. Frontend Component Changes

### New components

| Component | Description |
|-----------|-------------|
| `ConversationSidebar` | Left sidebar with conversation list, search, new chat button |
| `ConversationView` | Main chat area with messages, input, top bar |
| `DAGPanel` | Right panel with Wave groups, task selector, progress |
| `ParticipantList` | Expandable avatar list in top bar |
| `EmptyState` | Centered input for new conversations |

### Modified components

| Component | Change |
|-----------|--------|
| `GroupChat` | Refactor into `ConversationView` — remove task-specific logic |
| `ChatMessage` | Keep, add optional task-id left-border indicator |
| `SubtaskDetailPanel` | Keep, triggered from DAG panel step click |

### Removed/replaced

| Component | Replacement |
|-----------|-------------|
| `page.tsx` (dashboard) | `ConversationView` (chat-first home) |
| `NewTaskDialog` | Input in `ConversationView` (no modal needed) |
| `TaskCard` | Conversation list item in sidebar |
| `DAGView` (ReactFlow) | Keep as "expanded" view, triggered from DAG panel "↗" |

### Route changes

| Old Route | New Route | Description |
|-----------|-----------|-------------|
| `/` (dashboard) | `/` (latest conversation or empty state) | Chat is home |
| `/tasks/[id]` | `/c/[id]` | Conversation view |
| `/agents/*` | `/manage/agents/*` | Under manage prefix |
| `/templates/*` | `/manage/templates/*` | Under manage prefix |
| `/settings/*` | `/manage/settings/*` | Under manage prefix |
| `/analytics` | `/manage/analytics` | Under manage prefix |
| `/audit` | `/manage/audit` | Under manage prefix |

### Store changes

| Store | Change |
|-------|--------|
| New `useConversationStore` | Conversations list, current conversation, messages, SSE |
| `useTaskStore` | Simplified — only task detail, no longer manages messages |
| Remove `useOrgStore` | Already removed |

---

## 6. SSE Changes

Current: SSE per task (`/api/tasks/{id}/events`)
New: SSE per conversation (`/api/conversations/{id}/events`)

The conversation SSE stream includes ALL events from ALL tasks in the conversation. Frontend dispatches events by type:

- `message` → add to message list
- `task.created` → update DAG panel
- `subtask.*` → update DAG panel step status
- `agent.joined` → update participant list
- `approval.*` → show approval UI

---

## 7. Management Interface

Accessed via ⚙️ in the sidebar. Opens as a separate route (`/manage/*`) with its own nav:

```
/manage
  ├── /agents         (agent list)
  ├── /agents/health  (health dashboard)
  ├── /agents/[id]    (agent detail)
  ├── /agents/register
  ├── /templates      (template list)
  ├── /templates/[id] (template detail + editor)
  ├── /analytics      (analytics dashboard)
  ├── /audit          (audit log)
  ├── /settings/policies
  ├── /settings/webhooks
  └── /settings/a2a-server
```

The manage interface has its own sidebar nav (the current Nav component, adapted). A "← Back to Chat" link returns to the conversation view.

---

## 8. Open Source / Commercial Boundary

- **Open source:** Full chat UI, conversation management, multi-task conversations, DAG panel, basic management pages
- **Commercial:** Self-evolving template proposals, advanced analytics, audit log export, SSO
