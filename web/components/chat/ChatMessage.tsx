"use client";

import React, { useMemo } from "react";
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

function stripCodeFence(content: string): string | null {
  const match = content.trim().match(/^```(?:\w+)?\s*\n?([\s\S]*?)\n?\s*```$/);
  return match ? match[1] : null;
}

/**
 * Render inline text: **bold**, @mentions (new <@id|name> and legacy @word), and plain text.
 */
function renderInline(text: string, keyPrefix: string): React.ReactNode {
  // Split on **bold**, <@id|name> mentions, and @word mentions (legacy)
  const parts = text.split(/(\*\*[^*]+\*\*|<@[^>]+>|@\S+)/g);
  return parts.map((part, i) => {
    if (part.startsWith("**") && part.endsWith("**")) {
      return (
        <strong key={`${keyPrefix}-${i}`} className="font-semibold text-foreground">
          {part.slice(2, -2)}
        </strong>
      );
    }
    // New format: <@agent_id|Display Name>
    const mentionMatch = part.match(/^<@([^|]+)\|([^>]+)>$/);
    if (mentionMatch) {
      return (
        <span key={`${keyPrefix}-${i}`} className="rounded bg-blue-500/20 px-1 py-0.5 font-semibold text-blue-400">
          @{mentionMatch[2]}
        </span>
      );
    }
    // Legacy: @word
    if (part.startsWith("@")) {
      return (
        <span key={`${keyPrefix}-${i}`} className="font-bold text-purple-400">
          {part}
        </span>
      );
    }
    return <React.Fragment key={`${keyPrefix}-${i}`}>{part}</React.Fragment>;
  });
}

/**
 * Simple markdown renderer. Handles:
 * - ## Headings (h2, h3, h4)
 * - **bold** inline
 * - - bullet lists
 * - 1. numbered lists
 * - | tables |
 * - --- horizontal rules
 * - @mentions
 * - Plain text with line breaks
 *
 * No external dependencies.
 */
function renderMarkdown(content: string): React.ReactNode {
  const lines = content.split("\n");
  const elements: React.ReactNode[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];
    const trimmed = line.trim();

    // Empty line → spacer
    if (trimmed === "") {
      i++;
      continue;
    }

    // Horizontal rule
    if (/^-{3,}$/.test(trimmed)) {
      elements.push(<hr key={i} className="my-2 border-gray-700" />);
      i++;
      continue;
    }

    // Headings
    if (trimmed.startsWith("#### ")) {
      elements.push(
        <h4 key={i} className="mt-3 mb-1 text-sm font-semibold text-foreground">
          {renderInline(trimmed.slice(5), `h4-${i}`)}
        </h4>,
      );
      i++;
      continue;
    }
    if (trimmed.startsWith("### ")) {
      elements.push(
        <h3 key={i} className="mt-3 mb-1 text-sm font-bold text-foreground">
          {renderInline(trimmed.slice(4), `h3-${i}`)}
        </h3>,
      );
      i++;
      continue;
    }
    if (trimmed.startsWith("## ")) {
      elements.push(
        <h2 key={i} className="mt-4 mb-1.5 text-base font-bold text-foreground">
          {renderInline(trimmed.slice(3), `h2-${i}`)}
        </h2>,
      );
      i++;
      continue;
    }

    // Table (consecutive lines starting with |)
    if (trimmed.startsWith("|")) {
      const tableLines: string[] = [];
      while (i < lines.length && lines[i].trim().startsWith("|")) {
        tableLines.push(lines[i].trim());
        i++;
      }
      // Parse table
      const rows = tableLines
        .filter((l) => !/^\|[\s-|]+\|$/.test(l)) // skip separator rows
        .map((l) =>
          l
            .split("|")
            .slice(1, -1) // remove empty first/last from split
            .map((cell) => cell.trim()),
        );
      if (rows.length > 0) {
        const [header, ...body] = rows;
        elements.push(
          <div key={`table-${i}`} className="my-2 overflow-auto">
            <table className="w-full text-xs border-collapse">
              <thead>
                <tr className="border-b border-gray-700">
                  {header.map((cell, ci) => (
                    <th
                      key={ci}
                      className="px-2 py-1 text-left font-semibold text-foreground"
                    >
                      {renderInline(cell, `th-${i}-${ci}`)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {body.map((row, ri) => (
                  <tr key={ri} className="border-b border-gray-800">
                    {row.map((cell, ci) => (
                      <td key={ci} className="px-2 py-1">
                        {renderInline(cell, `td-${i}-${ri}-${ci}`)}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>,
        );
      }
      continue;
    }

    // Unordered list (collect consecutive - lines)
    if (trimmed.startsWith("- ")) {
      const items: string[] = [];
      while (i < lines.length && lines[i].trim().startsWith("- ")) {
        items.push(lines[i].trim().slice(2));
        i++;
      }
      elements.push(
        <ul key={`ul-${i}`} className="my-1 ml-4 list-disc space-y-0.5">
          {items.map((item, li) => (
            <li key={li}>{renderInline(item, `li-${i}-${li}`)}</li>
          ))}
        </ul>,
      );
      continue;
    }

    // Ordered list (collect consecutive N. lines)
    if (/^\d+\.\s/.test(trimmed)) {
      const items: string[] = [];
      while (i < lines.length && /^\d+\.\s/.test(lines[i].trim())) {
        items.push(lines[i].trim().replace(/^\d+\.\s/, ""));
        i++;
      }
      elements.push(
        <ol key={`ol-${i}`} className="my-1 ml-4 list-decimal space-y-0.5">
          {items.map((item, li) => (
            <li key={li}>{renderInline(item, `oli-${i}-${li}`)}</li>
          ))}
        </ol>,
      );
      continue;
    }

    // Regular paragraph
    elements.push(
      <p key={i} className="my-1">
        {renderInline(trimmed, `p-${i}`)}
      </p>,
    );
    i++;
  }

  return <>{elements}</>;
}

/**
 * Renders message content:
 * - JSON → code block
 * - Code fences → code block
 * - Everything else → markdown
 */
function renderContent(content: string): React.ReactNode {
  // Check for markdown code fences first
  const fenceContent = stripCodeFence(content);
  if (fenceContent !== null) {
    const parsed = tryParseJSON(fenceContent);
    const displayText =
      parsed !== null ? JSON.stringify(parsed, null, 2) : fenceContent;
    return (
      <pre className="mt-1 max-h-48 max-w-full overflow-auto rounded bg-gray-900 p-2 text-xs text-gray-300">
        <code>{displayText}</code>
      </pre>
    );
  }

  // Check if entire content is JSON
  const parsed = tryParseJSON(content);
  if (parsed !== null) {
    return (
      <pre className="mt-1 max-h-48 max-w-full overflow-auto rounded bg-gray-900 p-2 text-xs text-gray-300">
        <code>{JSON.stringify(parsed, null, 2)}</code>
      </pre>
    );
  }

  // Render as markdown (handles plain text too — markdown is superset of plain text)
  return renderMarkdown(content);
}

export function ChatMessage({ message }: ChatMessageProps) {
  const initial = useMemo(
    () => (message.sender_name ? message.sender_name[0].toUpperCase() : "?"),
    [message.sender_name],
  );

  const avatarColor = senderColors[message.sender_type] ?? senderColors.system;

  const isAdvisory = useMemo(() => {
    if (!message.metadata) return false;
    return (message.metadata as Record<string, unknown>).advisory === true;
  }, [message.metadata]);

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
          {isAdvisory && (
            <span className="rounded-full bg-blue-500/10 px-2 py-0.5 text-[10px] font-medium text-blue-400">
              Advisory reply
            </span>
          )}
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
