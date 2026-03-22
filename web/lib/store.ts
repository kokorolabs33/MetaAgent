"use client";

import { create } from "zustand";
import { api } from "./api";
import { connectSSE } from "./sse";
import type {
  Agent,
  Channel,
  ChannelAgent,
  Message,
  SSEEvent,
  Task,
} from "./types";

interface TaskHubStore {
  // State
  agents: Agent[];
  tasks: Task[];
  currentTask: Task | null;
  currentChannel: Channel | null;
  messages: Message[];
  channelAgents: ChannelAgent[];
  channelId: string | null;
  isLoading: boolean;
  sseDisconnect: (() => void) | null;

  // Actions
  loadAgents: () => Promise<void>;
  loadTasks: () => Promise<void>;
  selectTask: (task: Task) => Promise<void>;
  submitTask: (description: string) => Promise<void>;
  connectToChannel: (channelId: string) => void;
  handleSSEEvent: (event: SSEEvent) => void;
  reset: () => void;
}

export const useTaskHubStore = create<TaskHubStore>((set, get) => ({
  agents: [],
  tasks: [],
  currentTask: null,
  currentChannel: null,
  messages: [],
  channelAgents: [],
  channelId: null,
  isLoading: false,
  sseDisconnect: null,

  loadAgents: async () => {
    try {
      const agents = await api.agents.list();
      set({ agents });
    } catch (e) {
      console.error("loadAgents:", e);
    }
  },

  loadTasks: async () => {
    try {
      const tasks = await api.tasks.list();
      // Deduplicate by id (React StrictMode calls effects twice in dev)
      const seen = new Set<string>();
      const uniqueTasks = tasks.filter((t) => seen.has(t.id) ? false : (seen.add(t.id), true));
      set({ tasks: uniqueTasks });
      // Auto-connect to the latest task that has a channel
      const { currentTask, connectToChannel } = get();
      if (!currentTask && tasks.length > 0) {
        const latest = tasks[0]; // API returns newest first
        try {
          const { channel_id } = await api.tasks.getChannel(latest.id);
          set({ currentTask: latest });
          connectToChannel(channel_id);
        } catch {
          // No channel yet (pending task), just show the task
          set({ currentTask: latest });
        }
      }
    } catch (e) {
      console.error("loadTasks:", e);
    }
  },

  selectTask: async (task: Task) => {
    get().reset();
    set({ currentTask: task, isLoading: true });
    try {
      const { channel_id } = await api.tasks.getChannel(task.id);
      get().connectToChannel(channel_id);
    } catch {
      set({ isLoading: false });
    }
  },

  submitTask: async (description: string) => {
    get().reset();
    set({ isLoading: true });
    try {
      const task = await api.tasks.create({ description });
      set((state) => ({
        currentTask: task,
        tasks: state.tasks.some((t) => t.id === task.id) ? state.tasks : [task, ...state.tasks],
      }));
      // Poll for channel until master agent creates it
      pollForChannel(task.id, get().connectToChannel);
    } catch (e) {
      console.error("submitTask:", e);
      set({ isLoading: false });
    }
  },

  connectToChannel: (channelId: string) => {
    const { sseDisconnect } = get();
    if (sseDisconnect) sseDisconnect();

    set({ channelId });

    // First fetch snapshot, THEN connect SSE so we don't race with catchup events
    api.channels.get(channelId).then((detail) => {
      set({
        currentChannel: detail.channel,
        messages: detail.messages,
        channelAgents: detail.agents,
        isLoading: false,
      });

      // Connect SSE only after we have the snapshot — catchup events from backend
      // will overlap with snapshot but handleSSEEvent handles idempotently
      const disconnect = connectSSE(
        api.channels.streamUrl(channelId),
        get().handleSSEEvent
      );
      set({ sseDisconnect: disconnect });
    }).catch((e) => {
      console.error("connectToChannel: channel fetch failed:", e);
      set({ isLoading: false });
    });
  },

  handleSSEEvent: (event: SSEEvent) => {
    switch (event.type) {
      case "task_started": {
        // Task is now running — backend will send channel_created shortly
        break;
      }
      case "channel_created": {
        const data = event.data as { channel_id: string };
        api.channels.get(data.channel_id).then((detail) => {
          set({
            currentChannel: detail.channel,
            messages: detail.messages,
            channelAgents: detail.agents,
          });
        });
        break;
      }
      case "agent_joined": {
        const data = event.data as { agent_id: string; agent_name: string };
        set((state) => {
          if (!state.channelId) return state;
          const exists = state.channelAgents.some(
            (ca) => ca.agent_id === data.agent_id
          );
          if (exists) return state;
          return {
            channelAgents: [
              ...state.channelAgents,
              {
                channel_id: state.channelId,
                agent_id: data.agent_id,
                status: "idle" as const,
              },
            ],
          };
        });
        break;
      }
      case "agent_working": {
        const data = event.data as { agent_id: string };
        set((state) => ({
          channelAgents: state.channelAgents.map((ca) =>
            ca.agent_id === data.agent_id ? { ...ca, status: "working" as const } : ca
          ),
        }));
        break;
      }
      case "agent_done": {
        const data = event.data as { agent_id: string };
        set((state) => ({
          channelAgents: state.channelAgents.map((ca) =>
            ca.agent_id === data.agent_id ? { ...ca, status: "done" as const } : ca
          ),
        }));
        break;
      }
      case "message": {
        const data = event.data as { message: Message };
        set((state) => ({
          messages: [...state.messages, data.message],
        }));
        break;
      }
      case "document_updated": {
        const data = event.data as { document: string };
        set((state) => ({
          currentChannel: state.currentChannel
            ? { ...state.currentChannel, document: data.document }
            : null,
        }));
        break;
      }
      case "task_completed": {
        const data = event.data as { task: Task };
        set({ currentTask: data.task });
        break;
      }
    }
  },

  reset: () => {
    const { sseDisconnect } = get();
    if (sseDisconnect) sseDisconnect();
    set({
      currentTask: null,
      currentChannel: null,
      messages: [],
      channelAgents: [],
      channelId: null,
      sseDisconnect: null,
      isLoading: false,
    });
  },
}));

// Poll GET /api/tasks/:id/channel until master agent creates the channel
function pollForChannel(
  taskId: string,
  connectToChannel: (channelId: string) => void
): void {
  let attempts = 0;
  const interval = setInterval(async () => {
    attempts++;
    if (attempts > 120) {
      clearInterval(interval);
      console.error("pollForChannel: timed out waiting for channel");
      return;
    }
    try {
      const { channel_id } = await api.tasks.getChannel(taskId);
      clearInterval(interval);
      connectToChannel(channel_id);
    } catch {
      // 404 expected until master agent creates channel — keep polling
    }
  }, 500);
}
