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
import { ChatMessage } from "./ChatMessage";
import { useTaskStore } from "@/lib/store";
import type { Message, SubTask } from "@/lib/types";

interface GroupChatProps {
  taskId: string;
  messages: Message[];
  agents: { id: string; name: string }[];
  subtasks?: SubTask[];
}

export function GroupChat({
  taskId,
  messages,
  agents,
  subtasks,
}: GroupChatProps) {
  const [input, setInput] = useState("");
  const [isSending, setIsSending] = useState(false);
  const [showMentions, setShowMentions] = useState(false);
  const [mentionFilter, setMentionFilter] = useState("");
  const [mentionIndex, setMentionIndex] = useState(0);

  const feedRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  const sendMessage = useTaskStore((s) => s.sendMessage);

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    if (feedRef.current) {
      feedRef.current.scrollTop = feedRef.current.scrollHeight;
    }
  }, [messages.length]);

  // Check if any subtask is waiting for input
  const hasWaitingSubtask = useMemo(
    () => subtasks?.some((st) => st.status === "input_required") ?? false,
    [subtasks],
  );

  // Filtered agents for @mention autocomplete
  const filteredAgents = useMemo(() => {
    if (!mentionFilter) return agents;
    const lower = mentionFilter.toLowerCase();
    return agents.filter((a) => a.name.toLowerCase().includes(lower));
  }, [agents, mentionFilter]);

  const handleInputChange = useCallback(
    (e: React.ChangeEvent<HTMLTextAreaElement>) => {
      const value = e.target.value;
      setInput(value);

      // Check for @mention trigger
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
        const newValue = `${beforeAt}@${agent.name} ${afterCursor}`;
        setInput(newValue);
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

    let content = trimmed;
    for (const [name, agent] of mentionMap) {
      content = content.replace(
        new RegExp(`@${name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}`, "g"),
        `<@${agent.id}|${agent.name}>`,
      );
    }

    setIsSending(true);
    try {
      await sendMessage(taskId, content);
      setInput("");
      setMentionMap(new Map());
    } catch {
      // Error handling is managed by the store
    } finally {
      setIsSending(false);
    }
  }, [input, isSending, sendMessage, taskId, mentionMap]);

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

      // Submit on Enter (without Shift)
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        void handleSubmit();
      }
    },
    [showMentions, filteredAgents, mentionIndex, insertMention, handleSubmit],
  );

  return (
    <div className="flex h-full flex-col">
      {/* Waiting for input banner */}
      {hasWaitingSubtask && (
        <div className="flex items-center gap-2 border-b border-amber-500/30 bg-amber-500/10 px-4 py-2">
          <AlertCircle className="size-4 text-amber-400" />
          <span className="text-sm text-amber-300">
            Waiting for your input — a subtask needs your response
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
          messages.map((msg) => <ChatMessage key={msg.id} message={msg} />)
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
            placeholder="Type a message... Use @ to mention an agent"
            rows={1}
            className="flex-1 resize-none rounded-lg border border-input bg-transparent px-3 py-2 text-sm transition-colors placeholder:text-muted-foreground focus-visible:border-ring focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
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
  );
}
