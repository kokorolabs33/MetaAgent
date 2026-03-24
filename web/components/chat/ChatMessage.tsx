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

function formatTime(dateStr: string | undefined): string {
  if (!dateStr) return "";
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return "";
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

/**
 * Detect if a string is valid JSON.
 */
function tryParseJSON(str: string): unknown | null {
  const trimmed = str.trim();
  if (
    (trimmed.startsWith("{") && trimmed.endsWith("}")) ||
    (trimmed.startsWith("[") && trimmed.endsWith("]"))
  ) {
    try {
      return JSON.parse(trimmed);
    } catch {
      return null;
    }
  }
  return null;
}

/**
 * Strip markdown code fences (```json ... ``` or ``` ... ```) and return inner content.
 * Returns null if no code fence is found.
 */
function stripCodeFence(content: string): string | null {
  const match = content.trim().match(/^```(?:\w+)?\s*\n?([\s\S]*?)\n?\s*```$/);
  return match ? match[1] : null;
}

/**
 * Renders inline text with @mentions highlighted and \n converted to <br>.
 */
function renderInlineText(text: string): React.ReactNode[] {
  // Split on @mentions and newlines
  const parts = text.split(/(@\w+|\n)/g);
  return parts.map((part, i) => {
    if (part === "\n") {
      return <br key={i} />;
    }
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

/**
 * Renders message content with smart formatting:
 * - JSON objects/arrays: pretty-printed in a code block
 * - Markdown code fences: rendered as a code block
 * - Plain text: @mentions highlighted, \n rendered as line breaks
 */
function renderContent(content: string): React.ReactNode {
  // Check for markdown code fences first
  const fenceContent = stripCodeFence(content);
  if (fenceContent !== null) {
    // Try to pretty-print if the fence content is JSON
    const parsed = tryParseJSON(fenceContent);
    const displayText = parsed !== null ? JSON.stringify(parsed, null, 2) : fenceContent;
    return (
      <pre className="mt-1 max-h-48 max-w-full overflow-auto rounded bg-gray-900 p-2 text-xs text-gray-300">
        <code>{displayText}</code>
      </pre>
    );
  }

  // Check if the entire content is JSON
  const parsed = tryParseJSON(content);
  if (parsed !== null) {
    return (
      <pre className="mt-1 max-h-48 max-w-full overflow-auto rounded bg-gray-900 p-2 text-xs text-gray-300">
        <code>{JSON.stringify(parsed, null, 2)}</code>
      </pre>
    );
  }

  // Plain text: render with @mentions and line breaks
  return renderInlineText(content);
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
          <div className="mt-0.5 text-xs italic text-gray-500">
            {renderContent(message.content)}
          </div>
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
        <div className="mt-0.5 max-w-full overflow-hidden text-sm leading-relaxed text-gray-300">
          {renderContent(message.content)}
        </div>
      </div>
    </div>
  );
}
