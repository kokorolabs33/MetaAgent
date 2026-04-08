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
import { ChatMessage, StreamingChatMessage } from "./ChatMessage";
import { ToolCallStatus } from "./ToolCallStatus";
import { TypingIndicator } from "./TypingIndicator";
import { AgentStatusDot } from "@/components/agent/AgentStatusDot";
import { cn } from "@/lib/utils";
import { useTaskStore, useAgentStore } from "@/lib/store";
import type { Message, SubTask, ToolCallEvent } from "@/lib/types";

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
  const [advisoryErrors, setAdvisoryErrors] = useState<string[]>([]);

  const feedRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  const sendMessage = useTaskStore((s) => s.sendMessage);
  const typingAgents = useTaskStore((s) => s.typingAgents);
  const streamingMessages = useTaskStore((s) => s.streamingMessages);
  const toolCallEvents = useTaskStore((s) => s.toolCallEvents);
  const allAgents = useAgentStore((s) => s.agents);

  // Auto-scroll to bottom on new messages, tool call events, or streaming updates
  const streamingCount = Object.keys(streamingMessages).length;
  const streamingContentLength = Object.values(streamingMessages).reduce(
    (acc, sm) => acc + sm.content.length,
    0,
  );
  useEffect(() => {
    if (feedRef.current) {
      feedRef.current.scrollTop = feedRef.current.scrollHeight;
    }
  }, [messages.length, toolCallEvents.length, streamingCount, streamingContentLength]);

  // Check if any subtask is waiting for input
  const hasWaitingSubtask = useMemo(
    () => subtasks?.some((st) => st.status === "input_required") ?? false,
    [subtasks],
  );

  // Derive agent activity status from subtasks (D-10)
  // When subtasks is undefined (not yet loaded), show all agents without status filtering
  const agentsWithStatus = useMemo(() => {
    return agents.map((agent) => {
      if (!subtasks || subtasks.length === 0) {
        // Subtasks not loaded yet — show all agents, assume potentially active
        return { ...agent, isActive: true, hasSubtask: true };
      }
      const agentSubtasks = subtasks.filter((st) => st.agent_id === agent.id);
      const hasSubtask = agentSubtasks.length > 0;
      const isActive = agentSubtasks.some(
        (st) => st.status === "running" || st.status === "input_required",
      );
      return { ...agent, isActive, hasSubtask };
    });
  }, [agents, subtasks]);

  // Filtered agents for @mention autocomplete (D-13: extend, not replace)
  // Only filter to participating agents when subtasks data is available
  const filteredAgents = useMemo(() => {
    const base = subtasks && subtasks.length > 0
      ? agentsWithStatus.filter((a) => a.hasSubtask)
      : agentsWithStatus; // Show all agents when subtasks not loaded
    if (!mentionFilter) return base;
    const lower = mentionFilter.toLowerCase();
    return base.filter((a) => a.name.toLowerCase().includes(lower));
  }, [agentsWithStatus, mentionFilter, subtasks]);

  // Merge messages and tool call events into a single chronological timeline
  const feedItems = useMemo(() => {
    const items: Array<{ type: "message"; data: Message } | { type: "tool"; data: ToolCallEvent }> = [
      ...messages.map((m) => ({ type: "message" as const, data: m })),
      ...toolCallEvents.map((e) => ({ type: "tool" as const, data: e })),
    ];
    items.sort((a, b) => {
      const aTime = a.data.created_at || "";
      const bTime = b.data.created_at || "";
      return aTime.localeCompare(bTime);
    });
    return items;
  }, [messages, toolCallEvents]);

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
    (agent: { id: string; name: string; isActive?: boolean }) => {
      // D-02: Block selection of inactive agents
      if ("isActive" in agent && !agent.isActive) {
        setAdvisoryErrors([`${agent.name} is not currently executing — advisory messages can only be sent to active agents`]);
        setShowMentions(false);
        setMentionFilter("");
        return;
      }

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
    setAdvisoryErrors([]); // Clear previous errors
    try {
      const errors = await sendMessage(taskId, content);
      setInput("");
      setMentionMap(new Map());
      if (errors && errors.length > 0) {
        setAdvisoryErrors(errors);
      }
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
        {feedItems.length === 0 ? (
          <div className="flex h-full items-center justify-center">
            <p className="text-sm text-muted-foreground">
              No messages yet. Start the conversation!
            </p>
          </div>
        ) : (
          feedItems.map((item) =>
            item.type === "message" ? (
              <ChatMessage key={item.data.id} message={item.data as Message} />
            ) : (
              <ToolCallStatus key={item.data.id} event={item.data as ToolCallEvent} />
            ),
          )
        )}

        {/* Streaming messages (Phase 9: real-time token display) */}
        {Object.values(streamingMessages).map((sm) => {
          // Resolve agent name from agent store (store only has agent_id)
          const resolvedName =
            allAgents.find((a) => a.id === sm.agent_id)?.name ?? sm.agent_name;
          return (
            <StreamingChatMessage
              key={`streaming-${sm.agent_id}`}
              agentId={sm.agent_id}
              agentName={resolvedName}
              content={sm.content}
            />
          );
        })}
      </div>

      {/* Typing indicators (D-07) */}
      {Object.entries(typingAgents).map(([agentId, agentName]) => (
        <TypingIndicator key={agentId} agentName={agentName} />
      ))}

      {/* Advisory errors (D-02) */}
      {advisoryErrors.length > 0 && (
        <div className="border-t border-red-500/20 bg-red-500/5 px-4 py-2">
          {advisoryErrors.map((err, i) => (
            <p key={i} className="text-xs text-red-400">{err}</p>
          ))}
        </div>
      )}

      {/* Input area */}
      <div className="relative border-t border-border p-3">
        {/* @mention dropdown */}
        {showMentions && filteredAgents.length > 0 && (
          <div className="absolute bottom-full left-3 right-3 mb-1 max-h-40 overflow-y-auto rounded-lg border border-border bg-card shadow-lg">
            {filteredAgents.map((agent, i) => (
              <button
                key={agent.id}
                type="button"
                className={cn(
                  "flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors",
                  i === mentionIndex
                    ? "bg-secondary text-foreground"
                    : "text-muted-foreground hover:bg-secondary/50",
                  !agent.isActive && "opacity-50",
                )}
                onMouseDown={(e) => {
                  e.preventDefault();
                  insertMention(agent);
                }}
              >
                <AgentStatusDot status={agent.isActive ? "working" : "idle"} size="sm" />
                <span>{agent.name}</span>
                {!agent.isActive && (
                  <span className="text-[10px] text-muted-foreground">(not active)</span>
                )}
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
