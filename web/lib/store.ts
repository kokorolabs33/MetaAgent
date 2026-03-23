"use client";

import { create } from "zustand";
import { api } from "./api";
import type {
  Organization,
  OrgListItem,
  Agent,
  Task,
  TaskWithSubtasks,
  SubTask,
  Message,
  TaskEvent,
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
  createTask: (
    orgId: string,
    title: string,
    description: string,
  ) => Promise<Task>;
  selectTask: (orgId: string, taskId: string) => Promise<void>;
  cancelTask: (orgId: string, taskId: string) => Promise<void>;
  sendMessage: (
    orgId: string,
    taskId: string,
    content: string,
  ) => Promise<void>;
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
      const statusMap: Record<string, Task["status"]> = {
        "task.planning": "planning",
        "task.running": "running",
        "task.completed": "completed",
        "task.failed": "failed",
        "task.cancelled": "cancelled",
      };
      const newStatus = statusMap[event.type];
      if (newStatus) {
        set({ currentTask: { ...currentTask, status: newStatus } });
      }
    }

    // Update subtask status from subtask lifecycle events
    if (event.type.startsWith("subtask.")) {
      const subtaskId = event.subtask_id;
      if (subtaskId) {
        const statusMap: Record<string, SubTask["status"]> = {
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
                st.id === subtaskId
                  ? { ...st, status: newStatus }
                  : st,
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
