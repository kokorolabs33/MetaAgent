"use client";

import React, { useMemo } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
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
 * Process React children to find and style @mention patterns.
 * Handles both <@id|name> format and legacy @word format.
 */
function renderMentions(children: React.ReactNode): React.ReactNode {
  return React.Children.map(children, (child) => {
    if (typeof child !== "string") {
      return child;
    }

    // Split on <@id|name> mentions and @word mentions
    const parts = child.split(/(<@[^>]+>|@\S+)/g);
    if (parts.length === 1) return child;

    return parts.map((part, i) => {
      // New format: <@agent_id|Display Name>
      const mentionMatch = part.match(/^<@([^|]+)\|([^>]+)>$/);
      if (mentionMatch) {
        return (
          <span
            key={i}
            className="rounded bg-blue-500/20 px-1 py-0.5 font-semibold text-blue-400"
          >
            @{mentionMatch[2]}
          </span>
        );
      }
      // Legacy: @word
      if (part.startsWith("@") && part.length > 1) {
        return (
          <span key={i} className="font-bold text-purple-400">
            {part}
          </span>
        );
      }
      return <React.Fragment key={i}>{part}</React.Fragment>;
    });
  });
}

/**
 * Renders markdown content using react-markdown with GFM support and syntax highlighting.
 */
function MessageContent({ content }: { content: string }) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      rehypePlugins={[rehypeHighlight]}
      components={{
        p: ({ children, ...props }) => (
          <p className="my-1" {...props}>
            {renderMentions(children)}
          </p>
        ),
        table: ({ children, ...props }) => (
          <div className="my-2 overflow-auto">
            <table className="w-full text-xs border-collapse" {...props}>
              {children}
            </table>
          </div>
        ),
        thead: ({ children, ...props }) => (
          <thead {...props}>{children}</thead>
        ),
        th: ({ children, ...props }) => (
          <th
            className="border-b border-gray-700 px-2 py-1 text-left font-semibold text-foreground"
            {...props}
          >
            {children}
          </th>
        ),
        td: ({ children, ...props }) => (
          <td className="border-b border-gray-800 px-2 py-1" {...props}>
            {children}
          </td>
        ),
        pre: ({ children, ...props }) => (
          <pre
            className="mt-1 max-h-64 max-w-full overflow-auto rounded bg-gray-900 p-3 text-xs text-gray-300"
            {...props}
          >
            {children}
          </pre>
        ),
        code: ({ className, children, ...props }) => {
          const isInline = !className;
          if (isInline) {
            return (
              <code
                className="rounded bg-gray-800 px-1 py-0.5 text-xs"
                {...props}
              >
                {children}
              </code>
            );
          }
          return (
            <code className={className} {...props}>
              {children}
            </code>
          );
        },
        h1: ({ children, ...props }) => (
          <h1
            className="mt-4 mb-1.5 text-lg font-bold text-foreground"
            {...props}
          >
            {children}
          </h1>
        ),
        h2: ({ children, ...props }) => (
          <h2
            className="mt-4 mb-1.5 text-base font-bold text-foreground"
            {...props}
          >
            {children}
          </h2>
        ),
        h3: ({ children, ...props }) => (
          <h3
            className="mt-3 mb-1 text-sm font-bold text-foreground"
            {...props}
          >
            {children}
          </h3>
        ),
        h4: ({ children, ...props }) => (
          <h4
            className="mt-3 mb-1 text-sm font-semibold text-foreground"
            {...props}
          >
            {children}
          </h4>
        ),
        ul: ({ children, ...props }) => (
          <ul className="my-1 ml-4 list-disc space-y-0.5" {...props}>
            {children}
          </ul>
        ),
        ol: ({ children, ...props }) => (
          <ol className="my-1 ml-4 list-decimal space-y-0.5" {...props}>
            {children}
          </ol>
        ),
        a: ({ children, href, ...props }) => (
          <a
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            className="text-blue-400 underline hover:text-blue-300"
            {...props}
          >
            {children}
          </a>
        ),
        hr: (props) => <hr className="my-2 border-gray-700" {...props} />,
        blockquote: ({ children, ...props }) => (
          <blockquote
            className="my-2 border-l-2 border-gray-600 pl-3 text-gray-400 italic"
            {...props}
          >
            {children}
          </blockquote>
        ),
        input: (props) => <input {...props} className="mr-1.5" disabled />,
      }}
    >
      {content}
    </ReactMarkdown>
  );
}

/**
 * Renders message content:
 * - Pure JSON -> formatted code block with JSON syntax highlighting
 * - Everything else -> markdown via react-markdown
 */
function renderContent(content: string): React.ReactNode {
  const trimmed = content.trim();
  if (
    (trimmed.startsWith("{") && trimmed.endsWith("}")) ||
    (trimmed.startsWith("[") && trimmed.endsWith("]"))
  ) {
    try {
      const parsed = JSON.parse(trimmed);
      const formatted = JSON.stringify(parsed, null, 2);
      return <MessageContent content={`\`\`\`json\n${formatted}\n\`\`\``} />;
    } catch {
      // Not valid JSON, fall through to markdown
    }
  }
  return <MessageContent content={content} />;
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
