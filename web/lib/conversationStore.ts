"use client";

import { create } from "zustand";
import { api } from "./api";
import { connectConversationSSE } from "./sse";
import type {
  Conversation,
  ConversationListItem,
  Message,
  Task,
  TaskEvent,
} from "./types";

interface ConversationStore {
  // Sidebar state
  conversations: ConversationListItem[];
  isLoadingList: boolean;

  // Active conversation state
  activeConversation: Conversation | null;
  messages: Message[];
  tasks: Task[];
  isLoading: boolean;
  sseDisconnect: (() => void) | null;

  // Actions
  loadConversations: () => Promise<void>;
  createConversation: () => Promise<Conversation>;
  selectConversation: (id: string) => Promise<void>;
  updateTitle: (id: string, title: string) => Promise<void>;
  deleteConversation: (id: string) => Promise<void>;
  sendMessage: (content: string) => Promise<void>;
  connectSSE: (id: string) => void;
  disconnectSSE: () => void;
  handleEvent: (event: TaskEvent) => void;
}

export const useConversationStore = create<ConversationStore>((set, get) => ({
  conversations: [],
  isLoadingList: false,

  activeConversation: null,
  messages: [],
  tasks: [],
  isLoading: false,
  sseDisconnect: null,

  loadConversations: async () => {
    set({ isLoadingList: true });
    try {
      const conversations = await api.conversations.list();
      set({ conversations, isLoadingList: false });
    } catch {
      set({ isLoadingList: false });
    }
  },

  createConversation: async () => {
    const conversation = await api.conversations.create();
    set((s) => ({
      conversations: [
        {
          id: conversation.id,
          title: conversation.title,
          agent_count: 0,
          task_count: 0,
          latest_status: "",
          updated_at: conversation.updated_at,
        },
        ...s.conversations,
      ],
    }));
    return conversation;
  },

  selectConversation: async (id: string) => {
    set({ isLoading: true });
    try {
      const [conversation, messages, tasks] = await Promise.all([
        api.conversations.get(id),
        api.conversations.messages(id),
        api.conversations.tasks(id),
      ]);
      set({
        activeConversation: conversation,
        messages,
        tasks,
        isLoading: false,
      });
    } catch {
      set({ isLoading: false });
    }
  },

  updateTitle: async (id: string, title: string) => {
    const updated = await api.conversations.update(id, { title });
    set((s) => ({
      activeConversation:
        s.activeConversation?.id === id
          ? { ...s.activeConversation, title: updated.title }
          : s.activeConversation,
      conversations: s.conversations.map((c) =>
        c.id === id ? { ...c, title: updated.title } : c,
      ),
    }));
  },

  deleteConversation: async (id: string) => {
    await api.conversations.delete(id);
    set((s) => ({
      conversations: s.conversations.filter((c) => c.id !== id),
      activeConversation:
        s.activeConversation?.id === id ? null : s.activeConversation,
      messages: s.activeConversation?.id === id ? [] : s.messages,
      tasks: s.activeConversation?.id === id ? [] : s.tasks,
    }));
  },

  sendMessage: async (content: string) => {
    const { activeConversation } = get();
    if (!activeConversation) return;

    const msg = await api.conversations.sendMessage(
      activeConversation.id,
      content,
    );
    // Optimistically add -- SSE dedup will skip if it arrives again
    set((s) => {
      if (s.messages.some((m) => m.id === msg.id)) return s;
      return { messages: [...s.messages, msg] };
    });
    // Refresh sidebar to pick up auto-generated title
    void get().loadConversations();
  },

  connectSSE: (id: string) => {
    get().disconnectSSE();
    const disconnect = connectConversationSSE(id, (event) => {
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

  handleEvent: (event: TaskEvent) => {
    // Handle task lifecycle events
    if (event.type === "task.created") {
      const data = event.data as Record<string, unknown>;
      const newTask: Task = {
        id: event.task_id,
        title: (data.title as string) ?? "Untitled",
        description: (data.description as string) ?? "",
        status: "pending",
        created_by: (data.created_by as string) ?? "",
        conversation_id: get().activeConversation?.id ?? "",
        replan_count: 0,
        source: (data.source as string) ?? "conversation",
        completed_subtasks: 0,
        total_subtasks: 0,
        agent_ids: [],
        created_at: event.created_at ?? new Date().toISOString(),
      };
      set((s) => {
        if (s.tasks.some((t) => t.id === newTask.id)) return s;
        return { tasks: [...s.tasks, newTask] };
      });
      // Refresh sidebar to update task count
      void get().loadConversations();
      return;
    }

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
            t.id === event.task_id
              ? {
                  ...t,
                  status: newStatus,
                  ...(newStatus === "completed" && event.data.result
                    ? { result: event.data.result as Record<string, unknown> }
                    : {}),
                }
              : t,
          ),
        }));
      }
    }

    // Handle subtask lifecycle events
    if (event.type.startsWith("subtask.")) {
      const subtaskId = event.subtask_id;
      const taskId = event.task_id;
      if (!subtaskId || !taskId) return;

      if (event.type === "subtask.created") {
        // Refresh tasks to get updated subtask list
        const { activeConversation } = get();
        if (activeConversation) {
          void api.conversations.tasks(activeConversation.id).then((tasks) => {
            set({ tasks });
          });
        }
        return;
      }

      // For other subtask events, we do not track subtasks in the conversation store
      // directly since tasks array is Task[] not TaskWithSubtasks[].
      // The DAGPanel will fetch subtasks separately.
    }

    // Handle message events
    if (event.type === "message") {
      const data = event.data as Record<string, unknown>;
      const msgId = data.message_id as string | undefined;
      if (data.content && msgId) {
        set((s) => {
          if (s.messages.some((m) => m.id === msgId)) return s;
          const msg: Message = {
            id: msgId,
            task_id: event.task_id ?? "",
            conversation_id: get().activeConversation?.id ?? "",
            sender_type:
              (data.sender_type as Message["sender_type"]) ??
              (event.actor_type as Message["sender_type"]) ??
              "system",
            sender_id:
              (data.sender_id as string | undefined) ?? event.actor_id,
            sender_name: (data.sender_name as string) ?? "Unknown",
            content: data.content as string,
            mentions: (data.mentions as string[]) ?? [],
            created_at:
              (data.created_at as string) ??
              event.created_at ??
              new Date().toISOString(),
          };
          return { messages: [...s.messages, msg] };
        });
      }
    }

    // Handle tool call events — render as inline system messages
    if (event.type === "tool.call_started") {
      const data = event.data as Record<string, unknown>;
      const toolName = (data.tool_name as string) || "unknown";
      const toolArgs = (data.args as string) || "";
      const toolMsgId = `tool-${event.subtask_id ?? event.task_id}-${toolName}-${Date.now()}`;
      const displayArgs = toolArgs.length > 120 ? toolArgs.slice(0, 120) + "..." : toolArgs;
      const content = toolName === "web_search"
        ? `\u{1F50D} Searching: "${displayArgs}"`
        : `\u{1F527} Using ${toolName}: ${displayArgs}`;
      const msg: Message = {
        id: toolMsgId,
        task_id: event.task_id ?? "",
        conversation_id: get().activeConversation?.id ?? "",
        sender_type: "system",
        sender_name: "Tool",
        content,
        mentions: [],
        metadata: { tool_event: true, tool_name: toolName, tool_status: "started" },
        created_at: event.created_at ?? new Date().toISOString(),
      };
      set((s) => ({ messages: [...s.messages, msg] }));
      return;
    }

    if (event.type === "tool.call_completed") {
      const data = event.data as Record<string, unknown>;
      const toolName = (data.tool_name as string) || "unknown";
      const summary = (data.summary as string) || "Done";
      const displaySummary = summary.length > 100 ? summary.slice(0, 100) + "..." : summary;
      // Try to update the most recent matching started message
      set((s) => {
        const idx = [...s.messages].reverse().findIndex(
          (m) =>
            m.metadata?.tool_event === true &&
            m.metadata?.tool_name === toolName &&
            m.metadata?.tool_status === "started",
        );
        if (idx !== -1) {
          const realIdx = s.messages.length - 1 - idx;
          const updated = [...s.messages];
          updated[realIdx] = {
            ...updated[realIdx],
            content: `\u2705 ${toolName === "web_search" ? "Search" : toolName} complete: ${displaySummary}`,
            metadata: { ...updated[realIdx].metadata, tool_status: "completed" },
          };
          return { messages: updated };
        }
        // No matching started message — add a standalone completion message
        const msg: Message = {
          id: `tool-done-${event.subtask_id ?? event.task_id}-${toolName}-${Date.now()}`,
          task_id: event.task_id ?? "",
          conversation_id: get().activeConversation?.id ?? "",
          sender_type: "system",
          sender_name: "Tool",
          content: `\u2705 ${toolName === "web_search" ? "Search" : toolName} complete: ${displaySummary}`,
          mentions: [],
          metadata: { tool_event: true, tool_name: toolName, tool_status: "completed" },
          created_at: event.created_at ?? new Date().toISOString(),
        };
        return { messages: [...s.messages, msg] };
      });
      return;
    }

    // Handle approval events
    if (event.type === "approval.requested") {
      // Refetch tasks to get the plan data for the DAG preview
      const { activeConversation } = get();
      if (activeConversation) {
        void api.conversations.tasks(activeConversation.id).then((tasks) => {
          set({ tasks });
        });
      } else {
        set((s) => ({
          tasks: s.tasks.map((t) =>
            t.id === event.task_id
              ? { ...t, status: "approval_required" as Task["status"] }
              : t,
          ),
        }));
      }
    }
    if (event.type === "approval.resolved") {
      const { activeConversation } = get();
      if (activeConversation) {
        void api.conversations.tasks(activeConversation.id).then((tasks) => {
          set({ tasks });
        });
      }
    }
  },
}));
