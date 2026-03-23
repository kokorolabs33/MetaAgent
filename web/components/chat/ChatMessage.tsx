"use client";

import { useMemo } from "react";
import { cn } from "@/lib/utils";
import type { Message } from "@/lib/types";

interface ChatMessageProps {
  message: Message;
}

const senderColors: Record<Message["sender_type"], string> = {
  system: "bg-gray-600",
  agent: "bg-blue-600",
  user: "bg-purple-600",
};

function formatTime(dateStr: string): string {
  const d = new Date(dateStr);
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

/**
 * Renders content with @mentions highlighted in bold purple text.
 */
function renderContent(content: string): React.ReactNode[] {
  const parts = content.split(/(@\w+)/g);
  return parts.map((part, i) => {
    if (part.startsWith("@")) {
      return (
        <span key={i} className="font-bold text-purple-400">
          {part}
        </span>
      );
    }
    return <span key={i}>{part}</span>;
  });
}

export function ChatMessage({ message }: ChatMessageProps) {
  const initial = useMemo(
    () => (message.sender_name ? message.sender_name[0].toUpperCase() : "?"),
    [message.sender_name],
  );

  const avatarColor = senderColors[message.sender_type] ?? senderColors.system;

  if (message.sender_type === "system") {
    return (
      <div className="flex items-start gap-3 px-4 py-2">
        <div
          className={cn(
            "flex size-7 shrink-0 items-center justify-center rounded-full text-xs font-bold text-white",
            avatarColor,
          )}
        >
          {initial}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-baseline gap-2">
            <span className="text-xs font-medium text-gray-500">
              {message.sender_name}
            </span>
            <span className="text-[10px] text-gray-600">
              {formatTime(message.created_at)}
            </span>
          </div>
          <p className="mt-0.5 text-xs italic text-gray-500">
            {renderContent(message.content)}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex items-start gap-3 px-4 py-2">
      <div
        className={cn(
          "flex size-7 shrink-0 items-center justify-center rounded-full text-xs font-bold text-white",
          avatarColor,
        )}
      >
        {initial}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline gap-2">
          <span className="text-sm font-medium text-foreground">
            {message.sender_name}
          </span>
          <span className="text-[10px] text-muted-foreground">
            {formatTime(message.created_at)}
          </span>
        </div>
        <p className="mt-0.5 text-sm leading-relaxed text-gray-300">
          {renderContent(message.content)}
        </p>
      </div>
    </div>
  );
}
