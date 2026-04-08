"use client";

import { useState, useCallback, useRef, useEffect, useMemo } from "react";
import { Pencil, Check, Users, X } from "lucide-react";
import { useConversationStore } from "@/lib/conversationStore";

export function TopBar() {
  const { activeConversation, messages, updateTitle } = useConversationStore();
  const [isEditing, setIsEditing] = useState(false);
  const [editValue, setEditValue] = useState("");
  const [showParticipants, setShowParticipants] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const title = activeConversation?.title ?? "New Conversation";

  const startEdit = useCallback(() => {
    setEditValue(title);
    setIsEditing(true);
  }, [title]);

  useEffect(() => {
    if (isEditing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [isEditing]);

  const saveTitle = useCallback(async () => {
    const trimmed = editValue.trim();
    if (trimmed && activeConversation && trimmed !== activeConversation.title) {
      await updateTitle(activeConversation.id, trimmed);
    }
    setIsEditing(false);
  }, [editValue, activeConversation, updateTitle]);

  const cancelEdit = useCallback(() => {
    setIsEditing(false);
  }, []);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter") {
        void saveTitle();
      } else if (e.key === "Escape") {
        cancelEdit();
      }
    },
    [saveTitle, cancelEdit],
  );

  // Build participants from messages
  const participants = useMemo(() => {
    const map = new Map<string, { name: string; type: string }>();
    for (const msg of messages) {
      if (msg.sender_type === "system") continue; // skip system messages
      const key = msg.sender_id ?? msg.sender_name;
      if (!map.has(key)) {
        map.set(key, { name: msg.sender_name, type: msg.sender_type });
      }
    }
    return Array.from(map.values());
  }, [messages]);

  const participantCount = Math.max(participants.length, 1);

  if (!activeConversation) return null;

  return (
    <div className="flex items-center justify-between border-b border-border px-4 py-3">
      {/* Title */}
      <div className="flex items-center gap-2 min-w-0">
        {isEditing ? (
          <div className="flex items-center gap-1.5">
            <input
              ref={inputRef}
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              onKeyDown={handleKeyDown}
              onBlur={() => void saveTitle()}
              className="rounded border border-input bg-transparent px-2 py-1 text-lg font-semibold text-foreground outline-none focus:border-ring focus:ring-1 focus:ring-ring"
            />
            <button
              onClick={() => void saveTitle()}
              className="rounded p-1 text-green-400 hover:bg-green-500/10"
            >
              <Check className="size-4" />
            </button>
            <button
              onClick={cancelEdit}
              className="rounded p-1 text-gray-400 hover:bg-gray-500/10"
            >
              <X className="size-4" />
            </button>
          </div>
        ) : (
          <div className="flex items-center gap-2 min-w-0">
            <h1 className="truncate text-lg font-semibold text-foreground">
              {title}
            </h1>
            <button
              onClick={startEdit}
              className="rounded p-1 text-muted-foreground transition-colors hover:text-foreground"
              title="Edit title"
            >
              <Pencil className="size-3.5" />
            </button>
          </div>
        )}
      </div>

      {/* Participants */}
      <div className="relative">
        <button
          onClick={() => setShowParticipants(!showParticipants)}
          className="flex items-center gap-1.5 rounded-lg px-2 py-1 text-sm text-muted-foreground transition-colors hover:text-foreground hover:bg-secondary/50"
        >
          <Users className="size-4" />
          <span>{participantCount}</span>
        </button>

        {showParticipants && (
          <div className="absolute right-0 top-full mt-1 z-20 w-56 rounded-lg border border-border bg-card p-2 shadow-lg">
            <div className="text-xs font-semibold text-muted-foreground px-2 py-1">
              Participants
            </div>
            {participants.length === 0 ? (
              <div className="px-2 py-1 text-xs text-muted-foreground">
                Just you
              </div>
            ) : (
              participants.map((p, i) => {
                const agentColors = [
                  "bg-blue-600",
                  "bg-cyan-600",
                  "bg-teal-600",
                  "bg-indigo-600",
                  "bg-sky-600",
                ];
                const avatarColor =
                  p.type === "user"
                    ? "bg-purple-600"
                    : agentColors[i % agentColors.length];
                return (
                  <div
                    key={`${p.type}-${p.name}`}
                    className="flex items-center gap-2 rounded px-2 py-1.5 text-sm"
                  >
                    <div
                      className={`flex size-6 items-center justify-center rounded-full ${avatarColor} text-[10px] font-bold text-white`}
                    >
                      {p.name[0]?.toUpperCase() ?? "?"}
                    </div>
                    <span className="truncate text-foreground">{p.name}</span>
                    <span className="ml-auto text-xs text-muted-foreground capitalize">
                      {p.type}
                    </span>
                  </div>
                );
              })
            )}
          </div>
        )}
      </div>
    </div>
  );
}
