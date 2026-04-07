"use client";

import { create } from "zustand";
import { api, isSendMessageResponse } from "./api";
import type {
  Agent,
  Task,
  TaskWithSubtasks,
  SubTask,
  Message,
  TaskEvent,
} from "./types";
import { connectSSE, connectAgentStatusSSE, connectMultiTaskSSE } from "./sse";
import type { AgentActivityStatus } from "./types";

// ─── Agent Store ────────────────────────────────────────────
interface AgentStore {
  agents: Agent[];
  isLoading: boolean;
  agentStatuses: Record<string, AgentActivityStatus>;
  statusSSEDisconnect: (() => void) | null;
  loadAgents: (q?: string) => Promise<void>;
  registerAgent: (data: Partial<Agent>) => Promise<Agent>;
  deleteAgent: (id: string) => Promise<void>;
  connectStatusSSE: () => void;
  disconnectStatusSSE: () => void;
  getAgentStatus: (agentId: string) => AgentActivityStatus;
}

export const useAgentStore = create<AgentStore>((set, get) => ({
  agents: [],
  isLoading: false,
  agentStatuses: {},
  statusSSEDisconnect: null,

  loadAgents: async (q?: string) => {
    set({ isLoading: true });
    try {
      const agents = await api.agents.list(q);
      set({ agents, isLoading: false });
    } catch {
      set({ isLoading: false });
    }
  },

  registerAgent: async (data) => {
    const agent = await api.agents.create(data);
    set((s) => ({ agents: [...s.agents, agent] }));
    return agent;
  },

  deleteAgent: async (id) => {
    await api.agents.delete(id);
    set((s) => ({ agents: s.agents.filter((a) => a.id !== id) }));
  },

  connectStatusSSE: () => {
    // Prevent duplicate connections
    get().disconnectStatusSSE();

    const disconnect = connectAgentStatusSSE((event) => {
      if (event.type === "agent.status_changed") {
        const data = event.data;
        const agentId = data.agent_id as string;
        const activityStatus = data.activity_status as AgentActivityStatus;
        if (agentId && activityStatus) {
          set((s) => ({
            agentStatuses: { ...s.agentStatuses, [agentId]: activityStatus },
          }));
        }
      }
    });
    set({ statusSSEDisconnect: disconnect });
  },

  disconnectStatusSSE: () => {
    const disconnect = get().statusSSEDisconnect;
    if (disconnect) {
      disconnect();
      set({ statusSSEDisconnect: null });
    }
  },

  getAgentStatus: (agentId: string): AgentActivityStatus => {
    return get().agentStatuses[agentId] ?? "unknown";
  },
}));

// ─── Task Store ─────────────────────────────────────────────
interface TaskStore {
  tasks: Task[];
  totalTasks: number;
  currentPage: number;
  totalPages: number;
  currentTask: TaskWithSubtasks | null;
  messages: Message[];
  isLoading: boolean;
  sseDisconnect: (() => void) | null;

  loadTasks: (params?: { status?: string; q?: string; page?: number }) => Promise<void>;
  createTask: (title: string, description: string, templateId?: string) => Promise<Task>;
  selectTask: (taskId: string) => Promise<void>;
  cancelTask: (taskId: string) => Promise<void>;
  sendMessage: (taskId: string, content: string) => Promise<void>;
  connectSSE: (taskId: string) => void;
  disconnectSSE: () => void;
  handleEvent: (event: TaskEvent) => void;
}

export const useTaskStore = create<TaskStore>((set, get) => ({
  tasks: [],
  totalTasks: 0,
  currentPage: 1,
  totalPages: 0,
  currentTask: null,
  messages: [],
  isLoading: false,
  sseDisconnect: null,

  loadTasks: async (params) => {
    set({ isLoading: true });
    try {
      const result = await api.tasks.list(params);
      set({
        tasks: result.items,
        totalTasks: result.total,
        currentPage: result.page,
        totalPages: result.pages,
        isLoading: false,
      });
    } catch {
      set({ isLoading: false });
    }
  },

  createTask: async (title, description, templateId?) => {
    const data: { title: string; description: string; template_id?: string } = { title, description };
    if (templateId) data.template_id = templateId;
    const task = await api.tasks.create(data);
    set((s) => ({ tasks: [task, ...s.tasks] }));
    return task;
  },

  selectTask: async (taskId) => {
    set({ isLoading: true });
    try {
      const [task, messages] = await Promise.all([
        api.tasks.get(taskId),
        api.messages.list(taskId),
      ]);
      set({ currentTask: task, messages, isLoading: false });
    } catch {
      set({ isLoading: false });
    }
  },

  cancelTask: async (taskId) => {
    await api.tasks.cancel(taskId);
    const task = get().currentTask;
    if (task && task.id === taskId) {
      set({ currentTask: { ...task, status: "cancelled" } });
    }
  },

  sendMessage: async (taskId, content) => {
    const result = await api.messages.send(taskId, content);
    // Handle response using type guard -- no unsafe casts needed
    const msg = isSendMessageResponse(result) ? result.message : result;
    // Optimistically add — SSE dedup will skip if it arrives again
    set((s) => {
      if (s.messages.some((m) => m.id === msg.id)) return s;
      return { messages: [...s.messages, msg] };
    });
  },

  connectSSE: (taskId) => {
    get().disconnectSSE();
    const disconnect = connectSSE(taskId, (event) => {
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
      const statusMap: Record<string, Task["status"]> = {
        "task.planning": "planning",
        "task.running": "running",
        "task.completed": "completed",
        "task.failed": "failed",
        "task.cancelled": "cancelled",
        "task.approval_required": "approval_required",
      };
      const newStatus = statusMap[event.type];
      if (newStatus) {
        set({ currentTask: { ...currentTask, status: newStatus } });
      }

      // Handle replanning — reload full task to get new subtasks (per D-09)
      if (event.type === "task.replanned") {
        void get().selectTask(currentTask.id);
        return;
      }
    }

    // Handle subtask lifecycle events
    if (event.type.startsWith("subtask.")) {
      const subtaskId = event.subtask_id;
      if (subtaskId) {
        // Handle subtask creation — add new subtask to the list
        if (event.type === "subtask.created") {
          const data = event.data as Record<string, unknown>;
          const subtasks = currentTask.subtasks ?? [];
          const exists = subtasks.some((st) => st.id === subtaskId);
          if (!exists) {
            const newSubtask: SubTask = {
              id: subtaskId,
              task_id: currentTask.id,
              agent_id: (data.agent_id as string) ?? "",
              instruction: (data.instruction as string) ?? "",
              depends_on: (data.depends_on as string[]) ?? [],
              status: "pending",
              attempt: 0,
              max_attempts: 3,
              created_at: event.created_at,
            };
            set({
              currentTask: {
                ...currentTask,
                subtasks: [...subtasks, newSubtask],
              },
            });
          }
          return;
        }

        // Handle subtask status updates
        const statusMap: Record<string, SubTask["status"]> = {
          "subtask.running": "running",
          "subtask.completed": "completed",
          "subtask.failed": "failed",
          "subtask.input_required": "input_required",
          "subtask.blocked": "blocked",
          "subtask.cancelled": "cancelled",
        };
        const newStatus = statusMap[event.type];
        if (newStatus) {
          const updatedSubtasks = (currentTask.subtasks ?? []).map((st) =>
            st.id === subtaskId
              ? { ...st, status: newStatus }
              : st,
          );
          set({
            currentTask: {
              ...currentTask,
              subtasks: updatedSubtasks,
            },
          });
        }
      }
    }

    // Add messages from message events (deduplicate by message_id)
    if (event.type === "message") {
      const data = event.data as Record<string, unknown>;
      const msgId = data.message_id as string | undefined;
      if (data.content && msgId) {
        set((s) => {
          // Skip if message already exists (from optimistic sendMessage or duplicate SSE)
          if (s.messages.some((m) => m.id === msgId)) {
            return s;
          }
          const msg: Message = {
            id: msgId,
            task_id: get().currentTask?.id ?? "",
            conversation_id: (data.conversation_id as string) ?? "",
            // sender_type: prefer data.sender_type, fallback to event.actor_type
            sender_type: (data.sender_type as Message["sender_type"])
              ?? (event.actor_type as Message["sender_type"])
              ?? "system",
            sender_id: (data.sender_id as string | undefined) ?? event.actor_id,
            sender_name: (data.sender_name as string) ?? "Unknown",
            content: data.content as string,
            mentions: (data.mentions as string[]) ?? [],
            created_at: (data.created_at as string) ?? event.created_at ?? new Date().toISOString(),
          };
          return { messages: [...s.messages, msg] };
        });
      }
    }

    // Handle approval lifecycle events
    if (event.type === "approval.requested") {
      set({
        currentTask: {
          ...currentTask,
          status: "approval_required" as Task["status"],
        },
      });
    }
    if (event.type === "approval.resolved") {
      void get().selectTask(currentTask.id);
    }

    // Update task result when task completes
    if (event.type === "task.completed" && event.data.result) {
      set({
        currentTask: {
          ...currentTask,
          status: "completed",
          result: event.data.result as Record<string, unknown>,
        },
      });
    }
  },
}));

// ─── Dashboard Store ────────────────────────────────────────

type TaskProgress = { completed: number; total: number };

interface DashboardStore {
  tasks: Task[];
  totalTasks: number;
  currentPage: number;
  totalPages: number;
  taskProgress: Record<string, TaskProgress>;
  isLoading: boolean;
  error: string | null;
  multiTaskSSEDisconnect: (() => void) | null;

  loadDashboard: (params?: { status?: string; q?: string; page?: number }) => Promise<void>;
  connectDashboardSSE: (taskIds: string[]) => void;
  disconnectDashboardSSE: () => void;
  handleDashboardEvent: (event: TaskEvent) => void;
}

export const useDashboardStore = create<DashboardStore>((set, get) => ({
  tasks: [],
  totalTasks: 0,
  currentPage: 1,
  totalPages: 0,
  taskProgress: {},
  isLoading: false,
  error: null,
  multiTaskSSEDisconnect: null,

  loadDashboard: async (params) => {
    set({ isLoading: true, error: null });
    try {
      const result = await api.tasks.list({
        ...params,
        per_page: 12, // UI-SPEC: 12 per page (4 rows × 3 cols)
      });
      // Build initial progress map from list response (Plan 01 extended
      // the query to return completed_subtasks + total_subtasks).
      const progress: Record<string, TaskProgress> = {};
      for (const t of result.items) {
        progress[t.id] = {
          completed: t.completed_subtasks,
          total: t.total_subtasks,
        };
      }
      set({
        tasks: result.items,
        totalTasks: result.total,
        currentPage: result.page,
        totalPages: result.pages,
        taskProgress: progress,
        isLoading: false,
      });
    } catch (err) {
      console.error("loadDashboard failed:", err);
      set({ isLoading: false, error: "Failed to load tasks. Check your connection and try again." });
    }
  },

  connectDashboardSSE: (taskIds) => {
    get().disconnectDashboardSSE();
    if (taskIds.length === 0) return;
    const disconnect = connectMultiTaskSSE(taskIds, (event) => {
      get().handleDashboardEvent(event as unknown as TaskEvent);
    });
    set({ multiTaskSSEDisconnect: disconnect });
  },

  disconnectDashboardSSE: () => {
    const disconnect = get().multiTaskSSEDisconnect;
    if (disconnect) {
      disconnect();
      set({ multiTaskSSEDisconnect: null });
    }
  },

  handleDashboardEvent: (event) => {
    const taskId = event.task_id;
    if (!taskId) return;

    // Task-level status updates: patch the task in the list.
    if (event.type.startsWith("task.")) {
      const statusMap: Record<string, Task["status"]> = {
        "task.planning": "planning",
        "task.running": "running",
        "task.completed": "completed",
        "task.failed": "failed",
        "task.cancelled": "cancelled",
        "task.approval_required": "approval_required",
      };
      const newStatus = statusMap[event.type];
      if (newStatus) {
        set((s) => ({
          tasks: s.tasks.map((t) =>
            t.id === taskId ? { ...t, status: newStatus } : t,
          ),
        }));
      }
      return;
    }

    // Subtask-level updates: maintain the progress map deltas.
    // Per research recommendation 4: subtask.completed AND subtask.failed
    // both count toward the progress bar's "completed" denominator because
    // they are both finalized states.
    if (event.type === "subtask.created") {
      set((s) => {
        const cur = s.taskProgress[taskId] ?? { completed: 0, total: 0 };
        return {
          taskProgress: {
            ...s.taskProgress,
            [taskId]: { completed: cur.completed, total: cur.total + 1 },
          },
        };
      });
      return;
    }
    if (event.type === "subtask.completed" || event.type === "subtask.failed") {
      set((s) => {
        const cur = s.taskProgress[taskId];
        if (!cur) return s;
        // Cap completed at total to avoid visual > 100% if SSE outruns list.
        const nextCompleted = Math.min(cur.completed + 1, cur.total);
        return {
          taskProgress: {
            ...s.taskProgress,
            [taskId]: { completed: nextCompleted, total: cur.total },
          },
        };
      });
      return;
    }
  },
}));
