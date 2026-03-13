"use client";

import { useEffect, useRef } from "react";
import { useTaskHubStore } from "@/lib/store";
import { ScrollArea } from "@/components/ui/scroll-area";
import { MessageCircle } from "lucide-react";

const SENDER_COLORS: Record<string, string> = {
  master: "#8b5cf6",
  user: "#6b7280",
};

export function MessageFeed() {
  const { messages, agents } = useTaskHubStore();
  const bottomRef = useRef<HTMLDivElement>(null);

  const agentColorMap = Object.fromEntries(agents.map((a) => [a.id, a.color]));

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center gap-2 px-4 py-2.5 border-b border-gray-800 bg-gray-900/50">
        <MessageCircle className="w-4 h-4 text-gray-400" />
        <span className="text-xs font-medium text-gray-400 uppercase tracking-wider">
          Agent Messages
        </span>
        <span className="ml-auto text-xs text-gray-600">{messages.length}</span>
      </div>
      <ScrollArea className="flex-1">
        <div className="px-4 py-3 space-y-3">
          {messages.length === 0 && (
            <div className="text-center text-gray-600 text-sm py-8">
              No messages yet
            </div>
          )}
          {messages.map((msg) => {
            const color =
              agentColorMap[msg.sender_id] ??
              SENDER_COLORS[msg.sender_id] ??
              "#6b7280";
            return (
              <div
                key={msg.id}
                className={`rounded-lg p-3 border ${
                  msg.type === "system"
                    ? "border-purple-800/40 bg-purple-900/10"
                    : msg.type === "result"
                    ? "border-gray-700/50 bg-gray-800/40"
                    : "border-gray-800 bg-gray-900/30"
                }`}
              >
                <div className="flex items-center gap-2 mb-1.5">
                  <div className="w-2 h-2 rounded-full" style={{ backgroundColor: color }} />
                  <span className="text-xs font-semibold" style={{ color }}>
                    {msg.sender_name}
                  </span>
                  <span className="text-xs text-gray-600 ml-auto">
                    {new Date(msg.created_at).toLocaleTimeString()}
                  </span>
                </div>
                <p className="text-sm text-gray-300 whitespace-pre-wrap leading-relaxed">
                  {msg.content}
                </p>
              </div>
            );
          })}
          <div ref={bottomRef} />
        </div>
      </ScrollArea>
    </div>
  );
}
