"use client";

import {
  useState,
  useEffect,
  useRef,
  useCallback,
  useMemo,
} from "react";
import { Send, AlertCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ChatMessage } from "@/components/chat/ChatMessage";
import { PlanReviewCard } from "./PlanReviewCard";
import { TopBar } from "./TopBar";
import { DAGPanel } from "./DAGPanel";
import { useConversationStore } from "@/lib/conversationStore";
import { useAgentStore } from "@/lib/store";
import { api } from "@/lib/api";

interface ConversationViewProps {
  conversationId: string;
}

export function ConversationView({ conversationId }: ConversationViewProps) {
  const {
    activeConversation,
    messages,
    tasks,
    isLoading,
    selectConversation,
    sendMessage,
    connectSSE,
    disconnectSSE,
  } = useConversationStore();
  const { agents: allAgents, loadAgents } = useAgentStore();

  const [input, setInput] = useState("");
  const [isSending, setIsSending] = useState(false);
  const [showMentions, setShowMentions] = useState(false);
  const [mentionFilter, setMentionFilter] = useState("");
  const [mentionIndex, setMentionIndex] = useState(0);

  const feedRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  // Load agents
  useEffect(() => {
    if (allAgents.length === 0) {
      void loadAgents();
    }
  }, [allAgents.length, loadAgents]);

  // Load conversation + connect SSE
  useEffect(() => {
    void selectConversation(conversationId);
    connectSSE(conversationId);

    return () => {
      disconnectSSE();
    };
  }, [conversationId, selectConversation, connectSSE, disconnectSSE]);

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    if (feedRef.current) {
      feedRef.current.scrollTop = feedRef.current.scrollHeight;
    }
  }, [messages.length]);

  // Build agent list for @mentions from available agents
  const agentList = useMemo(() => {
    return allAgents.map((a) => ({ id: a.id, name: a.name }));
  }, [allAgents]);

  const [isApproving, setIsApproving] = useState(false);

  // Check for tasks waiting for input
  const hasActiveTask = useMemo(
    () => tasks.some((t) => t.status === "running" || t.status === "planning"),
    [tasks],
  );

  // Check for tasks awaiting approval
  const approvalTask = useMemo(
    () => tasks.find((t) => t.status === "approval_required"),
    [tasks],
  );

  const handleApproval = useCallback(
    async (action: "approve" | "reject") => {
      if (!approvalTask) return;
      setIsApproving(true);
      try {
        await api.tasks.approve(approvalTask.id, action);
      } finally {
        setIsApproving(false);
      }
    },
    [approvalTask],
  );

  // Filtered agents for @mention autocomplete
  const filteredAgents = useMemo(() => {
    if (!mentionFilter) return agentList;
    const lower = mentionFilter.toLowerCase();
    return agentList.filter((a) => a.name.toLowerCase().includes(lower));
  }, [agentList, mentionFilter]);

  const handleInputChange = useCallback(
    (e: React.ChangeEvent<HTMLTextAreaElement>) => {
      const value = e.target.value;
      setInput(value);

      const cursorPos = e.target.selectionStart;
      const textBeforeCursor = value.slice(0, cursorPos);
      const atMatch = textBeforeCursor.match(/@(\w*)$/);

      if (atMatch) {
        setShowMentions(true);
        setMentionFilter(atMatch[1]);
        setMentionIndex(0);
      } else {
        setShowMentions(false);
        setMentionFilter("");
      }
    },
    [],
  );

  // Track mention mappings: display name → {id, name}
  const [mentionMap, setMentionMap] = useState<Map<string, { id: string; name: string }>>(new Map());

  const insertMention = useCallback(
    (agent: { id: string; name: string }) => {
      const textarea = inputRef.current;
      if (!textarea) return;

      const cursorPos = textarea.selectionStart;
      const textBeforeCursor = input.slice(0, cursorPos);
      const atMatch = textBeforeCursor.match(/@(\S*)$/);

      if (atMatch) {
        const beforeAt = textBeforeCursor.slice(
          0,
          textBeforeCursor.length - atMatch[0].length,
        );
        const afterCursor = input.slice(cursorPos);
        // Show readable @name in textarea, convert to <@id|name> on send
        const newValue = `${beforeAt}@${agent.name} ${afterCursor}`;
        setInput(newValue);
        // Track the mapping for conversion on send
        setMentionMap((prev) => {
          const next = new Map(prev);
          next.set(agent.name, agent);
          return next;
        });
      }

      setShowMentions(false);
      setMentionFilter("");
      textarea.focus();
    },
    [input],
  );

  const handleSubmit = useCallback(async () => {
    const trimmed = input.trim();
    if (!trimmed || isSending) return;

    // Convert @Display Name → <@id|Display Name> before sending
    let content = trimmed;
    for (const [name, agent] of mentionMap) {
      content = content.replace(
        new RegExp(`@${name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}`, "g"),
        `<@${agent.id}|${agent.name}>`,
      );
    }

    setIsSending(true);
    try {
      await sendMessage(content);
      setInput("");
      setMentionMap(new Map());
    } catch {
      // Error handling is managed by the store
    } finally {
      setIsSending(false);
    }
  }, [input, isSending, sendMessage, mentionMap]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (showMentions && filteredAgents.length > 0) {
        if (e.key === "ArrowDown") {
          e.preventDefault();
          setMentionIndex((i) =>
            i < filteredAgents.length - 1 ? i + 1 : 0,
          );
          return;
        }
        if (e.key === "ArrowUp") {
          e.preventDefault();
          setMentionIndex((i) =>
            i > 0 ? i - 1 : filteredAgents.length - 1,
          );
          return;
        }
        if (e.key === "Enter" || e.key === "Tab") {
          e.preventDefault();
          insertMention(filteredAgents[mentionIndex]);
          return;
        }
        if (e.key === "Escape") {
          e.preventDefault();
          setShowMentions(false);
          return;
        }
      }

      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        void handleSubmit();
      }
    },
    [showMentions, filteredAgents, mentionIndex, insertMention, handleSubmit],
  );

  if (isLoading && !activeConversation) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="size-6 animate-spin rounded-full border-2 border-muted-foreground border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <TopBar />

      <div className="flex flex-1 overflow-hidden">
        {/* Chat area */}
        <div className="flex flex-1 flex-col">
          {/* Active task banner */}
          {hasActiveTask && !approvalTask && (
            <div className="flex items-center gap-2 border-b border-blue-500/30 bg-blue-500/10 px-4 py-2">
              <AlertCircle className="size-4 text-blue-400" />
              <span className="text-sm text-blue-300">
                Task in progress -- agents are working
              </span>
            </div>
          )}

          {/* Message feed */}
          <div ref={feedRef} className="flex-1 overflow-y-auto py-2">
            {messages.length === 0 ? (
              <div className="flex h-full items-center justify-center">
                <p className="text-sm text-muted-foreground">
                  No messages yet. Start the conversation!
                </p>
              </div>
            ) : (
              messages.map((msg) => (
                <ChatMessage key={msg.id} message={msg} />
              ))
            )}
            {/* Plan review card inline in chat */}
            {approvalTask && (
              <PlanReviewCard
                task={approvalTask}
                isApproving={isApproving}
                onApprove={() => void handleApproval("approve")}
                onReject={() => void handleApproval("reject")}
              />
            )}
          </div>

          {/* Input area */}
          <div className="relative border-t border-border p-3">
            {/* @mention dropdown */}
            {showMentions && filteredAgents.length > 0 && (
              <div className="absolute bottom-full left-3 right-3 mb-1 max-h-40 overflow-y-auto rounded-lg border border-border bg-card shadow-lg">
                {filteredAgents.map((agent, i) => (
                  <button
                    key={agent.id}
                    type="button"
                    className={`flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors ${
                      i === mentionIndex
                        ? "bg-secondary text-foreground"
                        : "text-muted-foreground hover:bg-secondary/50"
                    }`}
                    onMouseDown={(e) => {
                      e.preventDefault();
                      insertMention(agent);
                    }}
                  >
                    <div className="flex size-5 items-center justify-center rounded-full bg-blue-600 text-[10px] font-bold text-white">
                      {agent.name[0]?.toUpperCase() ?? "?"}
                    </div>
                    <span>{agent.name}</span>
                  </button>
                ))}
              </div>
            )}

            <div className="flex items-end gap-2">
              <textarea
                ref={inputRef}
                value={input}
                onChange={handleInputChange}
                onKeyDown={handleKeyDown}
                placeholder="Describe a task or ask a question... Use @ to mention an agent"
                rows={3}
                className="flex-1 resize-none rounded-xl border border-input bg-transparent px-4 py-3 text-sm transition-colors placeholder:text-muted-foreground focus-visible:border-ring focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
              />
              <Button
                size="icon"
                onClick={() => void handleSubmit()}
                disabled={isSending || !input.trim()}
              >
                <Send className="size-4" />
              </Button>
            </div>
          </div>
        </div>

        {/* DAG panel (right side) */}
        <div className="w-72 shrink-0 border-l border-border">
          <DAGPanel />
        </div>
      </div>
    </div>
  );
}
