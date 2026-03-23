# Plan 4: Frontend

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the complete V2 frontend — Dashboard (task list), Task Detail (DAG pipeline + Group Chat), Agent Management (list, register, detail), and SSE integration for real-time updates.

**Architecture:** Next.js 15 app router. Zustand for state (3 stores). React Flow for DAG visualization. SSE via EventSource for real-time task events. shadcn/ui components.

**Tech Stack:** Next.js 15, React 19, Tailwind CSS 4, Zustand, React Flow, shadcn/ui

**Spec:** `docs/superpowers/specs/2026-03-22-taskhub-v2-design.md` (Sections 5)

**Depends on:** Plans 1-3 (backend complete)

---

## File Map

| File | Responsibility |
|------|----------------|
| `web/lib/api.ts` | V2 API client — add agent, task, message, subtask endpoints |
| `web/lib/sse.ts` | SSE connection manager with Last-Event-ID support |
| `web/lib/store.ts` | Zustand stores: TaskStore, TaskDetailStore, AgentStore |
| `web/app/page.tsx` | Dashboard — task list with cards, filters, new task dialog |
| `web/app/tasks/[id]/page.tsx` | Task detail — DAG pipeline + Group Chat |
| `web/app/agents/page.tsx` | Agent list |
| `web/app/agents/[id]/page.tsx` | Agent detail |
| `web/app/agents/register/page.tsx` | Agent registration wizard |
| `web/app/layout.tsx` | Root layout with nav sidebar |
| `web/components/dashboard/TaskCard.tsx` | Task card for dashboard list |
| `web/components/dashboard/NewTaskDialog.tsx` | Dialog to create a new task |
| `web/components/task/DAGView.tsx` | React Flow DAG pipeline visualization |
| `web/components/task/SubtaskNode.tsx` | Custom React Flow node for subtasks |
| `web/components/chat/GroupChat.tsx` | Group chat panel with message feed + input |
| `web/components/chat/ChatMessage.tsx` | Single chat message component |
| `web/components/agent/AgentCard.tsx` | Agent card for list view |
| `web/components/agent/AdapterForm.tsx` | Adapter config form (JSON editor) |
| `web/components/ui/nav.tsx` | Sidebar navigation component |

---

## Chunk 1: API Client & Stores

### Task 1: Update API client

**Files:**
- Modify: `web/lib/api.ts`

- [ ] **Step 1: Add agent, task, message endpoints**

Read the current api.ts first. Extend the `api` object with:

```typescript
agents: {
  list: (orgId: string) => get<Agent[]>(`/api/orgs/${orgId}/agents`),
  get: (orgId: string, id: string) => get<Agent>(`/api/orgs/${orgId}/agents/${id}`),
  create: (orgId: string, data: Partial<Agent>) => post<Agent>(`/api/orgs/${orgId}/agents`, data),
  update: (orgId: string, id: string, data: Partial<Agent>) => put<Agent>(`/api/orgs/${orgId}/agents/${id}`, data),
  delete: (orgId: string, id: string) => del(`/api/orgs/${orgId}/agents/${id}`),
  healthcheck: (orgId: string, id: string) => post<{status: number, latency_ms: number}>(`/api/orgs/${orgId}/agents/${id}/healthcheck`, {}),
},
tasks: {
  list: (orgId: string, status?: string) => {
    const params = status ? `?status=${status}` : '';
    return get<Task[]>(`/api/orgs/${orgId}/tasks${params}`);
  },
  get: (orgId: string, id: string) => get<TaskWithSubtasks>(`/api/orgs/${orgId}/tasks/${id}`),
  create: (orgId: string, data: {title: string, description: string}) => post<Task>(`/api/orgs/${orgId}/tasks`, data),
  cancel: (orgId: string, id: string) => post<void>(`/api/orgs/${orgId}/tasks/${id}/cancel`, {}),
  cost: (orgId: string, id: string) => get<{total_cost_usd: number, total_input_tokens: number, total_output_tokens: number}>(`/api/orgs/${orgId}/tasks/${id}/cost`),
  subtasks: (orgId: string, id: string) => get<SubTask[]>(`/api/orgs/${orgId}/tasks/${id}/subtasks`),
},
messages: {
  list: (orgId: string, taskId: string) => get<Message[]>(`/api/orgs/${orgId}/tasks/${taskId}/messages`),
  send: (orgId: string, taskId: string, content: string) => post<Message>(`/api/orgs/${orgId}/tasks/${taskId}/messages`, {content}),
},
```

Add the missing type imports: `Agent`, `Task`, `TaskWithSubtasks`, `SubTask`, `Message`.

- [ ] **Step 2: Commit**

```bash
git add web/lib/api.ts
git commit -m "feat(web): add agent, task, message API client endpoints"
```

---

### Task 2: SSE connection manager

**Files:**
- Modify: `web/lib/sse.ts`

- [ ] **Step 1: Write SSE manager with auto-reconnect and Last-Event-ID**

```typescript
// web/lib/sse.ts
const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export type SSEEventHandler = (event: { id: string; type: string; data: Record<string, unknown> }) => void;

export function connectSSE(
  orgId: string,
  taskId: string,
  onEvent: SSEEventHandler,
  onError?: (error: Event) => void
): () => void {
  const url = `${BASE}/api/orgs/${orgId}/tasks/${taskId}/events`;
  const eventSource = new EventSource(url, { withCredentials: true });

  eventSource.onmessage = (e) => {
    try {
      const parsed = JSON.parse(e.data);
      onEvent({
        id: e.lastEventId,
        type: parsed.type,
        data: parsed.data ?? parsed,
      });
    } catch {
      // ignore malformed events
    }
  };

  eventSource.onerror = (e) => {
    if (onError) onError(e);
    // EventSource auto-reconnects with Last-Event-ID header
  };

  // Return cleanup function
  return () => {
    eventSource.close();
  };
}
```

- [ ] **Step 2: Commit**

```bash
git add web/lib/sse.ts
git commit -m "feat(web): add SSE connection manager with auto-reconnect"
```

---

### Task 3: Zustand stores

**Files:**
- Modify: `web/lib/store.ts`

- [ ] **Step 1: Rewrite store with 3 slices**

```typescript
"use client";

import { create } from "zustand";
import { api } from "./api";
import type {
  Organization, OrgListItem, Agent, Task, TaskWithSubtasks,
  SubTask, Message, TaskEvent,
} from "./types";
import { connectSSE } from "./sse";

// ─── Org Store ──────────────────────────────────────────────
interface OrgStore {
  orgs: OrgListItem[];
  currentOrg: Organization | null;
  isLoading: boolean;
  loadOrgs: () => Promise<void>;
  selectOrg: (orgId: string) => Promise<void>;
}

export const useOrgStore = create<OrgStore>((set) => ({
  orgs: [],
  currentOrg: null,
  isLoading: false,
  loadOrgs: async () => {
    set({ isLoading: true });
    try {
      const orgs = await api.orgs.list();
      set({ orgs, isLoading: false });
    } catch {
      set({ isLoading: false });
    }
  },
  selectOrg: async (orgId) => {
    set({ isLoading: true });
    try {
      const org = await api.orgs.get(orgId);
      set({ currentOrg: org, isLoading: false });
    } catch {
      set({ isLoading: false });
    }
  },
}));

// ─── Agent Store ────────────────────────────────────────────
interface AgentStore {
  agents: Agent[];
  isLoading: boolean;
  loadAgents: (orgId: string) => Promise<void>;
  registerAgent: (orgId: string, data: Partial<Agent>) => Promise<Agent>;
  deleteAgent: (orgId: string, id: string) => Promise<void>;
}

export const useAgentStore = create<AgentStore>((set) => ({
  agents: [],
  isLoading: false,
  loadAgents: async (orgId) => {
    set({ isLoading: true });
    try {
      const agents = await api.agents.list(orgId);
      set({ agents, isLoading: false });
    } catch {
      set({ isLoading: false });
    }
  },
  registerAgent: async (orgId, data) => {
    const agent = await api.agents.create(orgId, data);
    set((s) => ({ agents: [...s.agents, agent] }));
    return agent;
  },
  deleteAgent: async (orgId, id) => {
    await api.agents.delete(orgId, id);
    set((s) => ({ agents: s.agents.filter((a) => a.id !== id) }));
  },
}));

// ─── Task Store ─────────────────────────────────────────────
interface TaskStore {
  tasks: Task[];
  currentTask: TaskWithSubtasks | null;
  messages: Message[];
  isLoading: boolean;
  sseDisconnect: (() => void) | null;

  loadTasks: (orgId: string, status?: string) => Promise<void>;
  createTask: (orgId: string, title: string, description: string) => Promise<Task>;
  selectTask: (orgId: string, taskId: string) => Promise<void>;
  cancelTask: (orgId: string, taskId: string) => Promise<void>;
  sendMessage: (orgId: string, taskId: string, content: string) => Promise<void>;
  connectSSE: (orgId: string, taskId: string) => void;
  disconnectSSE: () => void;
  handleEvent: (event: TaskEvent) => void;
}

export const useTaskStore = create<TaskStore>((set, get) => ({
  tasks: [],
  currentTask: null,
  messages: [],
  isLoading: false,
  sseDisconnect: null,

  loadTasks: async (orgId, status) => {
    set({ isLoading: true });
    try {
      const tasks = await api.tasks.list(orgId, status);
      set({ tasks, isLoading: false });
    } catch {
      set({ isLoading: false });
    }
  },

  createTask: async (orgId, title, description) => {
    const task = await api.tasks.create(orgId, { title, description });
    set((s) => ({ tasks: [task, ...s.tasks] }));
    return task;
  },

  selectTask: async (orgId, taskId) => {
    set({ isLoading: true });
    try {
      const [task, messages] = await Promise.all([
        api.tasks.get(orgId, taskId),
        api.messages.list(orgId, taskId),
      ]);
      set({ currentTask: task, messages, isLoading: false });
    } catch {
      set({ isLoading: false });
    }
  },

  cancelTask: async (orgId, taskId) => {
    await api.tasks.cancel(orgId, taskId);
    const task = get().currentTask;
    if (task && task.id === taskId) {
      set({ currentTask: { ...task, status: "cancelled" } });
    }
  },

  sendMessage: async (orgId, taskId, content) => {
    const msg = await api.messages.send(orgId, taskId, content);
    set((s) => ({ messages: [...s.messages, msg] }));
  },

  connectSSE: (orgId, taskId) => {
    get().disconnectSSE();
    const disconnect = connectSSE(orgId, taskId, (event) => {
      get().handleEvent(event as unknown as TaskEvent);
    });
    set({ sseDisconnect: disconnect });
  },

  disconnectSSE: () => {
    const disconnect = get().sseDisconnect;
    if (disconnect) {
      disconnect();
      set({ sseDisconnect: null });
    }
  },

  handleEvent: (event) => {
    const { currentTask } = get();
    if (!currentTask) return;

    // Update task status from task lifecycle events
    if (event.type.startsWith("task.")) {
      const statusMap: Record<string, string> = {
        "task.planning": "planning",
        "task.running": "running",
        "task.completed": "completed",
        "task.failed": "failed",
        "task.cancelled": "cancelled",
      };
      const newStatus = statusMap[event.type];
      if (newStatus) {
        set({ currentTask: { ...currentTask, status: newStatus as Task["status"] } });
      }
    }

    // Update subtask status from subtask lifecycle events
    if (event.type.startsWith("subtask.")) {
      const subtaskId = event.subtask_id;
      if (subtaskId) {
        const statusMap: Record<string, string> = {
          "subtask.running": "running",
          "subtask.completed": "completed",
          "subtask.failed": "failed",
          "subtask.waiting_for_input": "waiting_for_input",
          "subtask.blocked": "blocked",
          "subtask.cancelled": "cancelled",
        };
        const newStatus = statusMap[event.type];
        if (newStatus) {
          set({
            currentTask: {
              ...currentTask,
              subtasks: currentTask.subtasks.map((st) =>
                st.id === subtaskId ? { ...st, status: newStatus as SubTask["status"] } : st
              ),
            },
          });
        }
      }
    }

    // Add messages from message events
    if (event.type === "message") {
      const data = event.data as unknown as Message;
      if (data.content) {
        set((s) => ({ messages: [...s.messages, data] }));
      }
    }
  },
}));
```

- [ ] **Step 2: Commit**

```bash
git add web/lib/store.ts
git commit -m "feat(web): rewrite stores — OrgStore, AgentStore, TaskStore with SSE"
```

---

## Chunk 2: Clean Up V1 Components & Layout

### Task 4: Remove V1 components and set up V2 layout

**Files:**
- Delete: `web/components/channel/` (V1)
- Delete: `web/components/chat/MasterChatModal.tsx` (V1)
- Delete: `web/components/task/TaskBar.tsx` (V1)
- Delete: `web/components/task/TaskList.tsx` (V1)
- Delete: `web/components/topology/` (V1)
- Create: `web/components/ui/nav.tsx`
- Modify: `web/app/layout.tsx`

- [ ] **Step 1: Delete V1 component directories**

```bash
rm -rf web/components/channel web/components/chat web/components/task web/components/topology
mkdir -p web/components/dashboard web/components/task web/components/chat web/components/agent
```

- [ ] **Step 2: Create sidebar nav component**

`web/components/ui/nav.tsx` — simple sidebar with links to Dashboard (/), Agents (/agents). Show current org name. Highlight active route.

- [ ] **Step 3: Update layout.tsx**

Clean layout with sidebar nav on left, main content area on right. Dark theme. Import the nav component.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "chore(web): clean V1 components, add nav sidebar, update layout"
```

---

## Chunk 3: Dashboard Page

### Task 5: Dashboard — task list with cards

**Files:**
- Create: `web/components/dashboard/TaskCard.tsx`
- Create: `web/components/dashboard/NewTaskDialog.tsx`
- Modify: `web/app/page.tsx`

- [ ] **Step 1: TaskCard component**

Card showing: title, status badge (colored), subtask progress (e.g. "3/5"), time ago. Needs-input indicator (red dot). Click navigates to `/tasks/:id`.

Status colors:
- pending: gray
- planning: blue
- running: yellow
- completed: green
- failed: red
- cancelled: gray/strikethrough

- [ ] **Step 2: NewTaskDialog component**

Dialog with title + description inputs. Submit calls `taskStore.createTask()`. Navigates to task detail on success.

- [ ] **Step 3: Dashboard page**

Page with header ("Tasks" + filter dropdown + "New Task" button). List of TaskCards. Load tasks on mount. Show empty state if no tasks.

- [ ] **Step 4: Commit**

```bash
git add web/components/dashboard/ web/app/page.tsx
git commit -m "feat(web): add Dashboard — task cards, filters, new task dialog"
```

---

## Chunk 4: Task Detail Page

### Task 6: DAG pipeline visualization

**Files:**
- Create: `web/components/task/DAGView.tsx`
- Create: `web/components/task/SubtaskNode.tsx`

- [ ] **Step 1: SubtaskNode — custom React Flow node**

Display: agent name, instruction (truncated), status icon, progress bar (if running). Flash animation for waiting_for_input status. Colors match subtask status.

- [ ] **Step 2: DAGView — React Flow wrapper**

Convert subtasks + depends_on into React Flow nodes and edges. Auto-layout using dagre or simple top-to-bottom positioning. Handle click on node to scroll chat.

Import `@xyflow/react` and its CSS.

- [ ] **Step 3: Commit**

```bash
git add web/components/task/
git commit -m "feat(web): add DAG pipeline view with React Flow"
```

---

### Task 7: Group Chat

**Files:**
- Create: `web/components/chat/ChatMessage.tsx`
- Create: `web/components/chat/GroupChat.tsx`

- [ ] **Step 1: ChatMessage component**

Display sender avatar (colored circle with first letter), sender name, content, timestamp. System messages styled differently (italic, gray). @mentions highlighted. Agent messages have agent color. User messages aligned right.

- [ ] **Step 2: GroupChat component**

Message feed with auto-scroll. Input box at bottom with @mention autocomplete (list agent names). Submit calls `taskStore.sendMessage()`. Show "needs input" indicator when a subtask is waiting.

- [ ] **Step 3: Commit**

```bash
git add web/components/chat/
git commit -m "feat(web): add Group Chat — message feed with @mention input"
```

---

### Task 8: Task detail page

**Files:**
- Create: `web/app/tasks/[id]/page.tsx`

- [ ] **Step 1: Task detail page layout**

Split view: DAG on left (40%), Chat on right (60%). Status bar at bottom (cost, tokens, duration). Connect SSE on mount, disconnect on unmount. Load task + messages on mount.

Show task title, status, and cancel button (if running). Back link to dashboard.

- [ ] **Step 2: Commit**

```bash
git add web/app/tasks/
git commit -m "feat(web): add Task detail page — DAG + Chat split view with SSE"
```

---

## Chunk 5: Agent Management

### Task 9: Agent management pages

**Files:**
- Create: `web/components/agent/AgentCard.tsx`
- Create: `web/components/agent/AdapterForm.tsx`
- Create: `web/app/agents/page.tsx`
- Create: `web/app/agents/[id]/page.tsx`
- Create: `web/app/agents/register/page.tsx`

- [ ] **Step 1: AgentCard component**

Card showing: name, description, status badge, adapter type, capabilities tags, endpoint (truncated). Click navigates to detail. Healthcheck button.

- [ ] **Step 2: AdapterForm component**

Form for adapter config: select adapter type (http_poll/native), endpoint input, auth type selector, auth fields (conditional on type). For http_poll: body template JSON editor, poll config fields (status_path, status_map, etc.). Test Connection button.

- [ ] **Step 3: Agent list page (/agents)**

Page with header + "Register Agent" button. Grid of AgentCards. Load agents on mount.

- [ ] **Step 4: Agent detail page (/agents/:id)**

Show full agent config. Edit form. Delete button. Healthcheck status. Show recent tasks that used this agent.

- [ ] **Step 5: Agent register page (/agents/register)**

Multi-step form using AdapterForm. Step 1: basic info (name, description, capabilities). Step 2: adapter config. Step 3: test + confirm.

- [ ] **Step 6: Commit**

```bash
git add web/components/agent/ web/app/agents/
git commit -m "feat(web): add Agent management — list, detail, register wizard"
```

---

## Chunk 6: Verify

### Task 10: End-to-end verification

- [ ] **Step 1: Run frontend lint**

```bash
cd web && pnpm lint
```

- [ ] **Step 2: Run type check**

```bash
cd web && pnpm tsc --noEmit
```

- [ ] **Step 3: Run build**

```bash
cd web && pnpm build
```

- [ ] **Step 4: Fix any issues and commit**

```bash
git add -A
git commit -m "fix(web): resolve lint and type errors"
```
