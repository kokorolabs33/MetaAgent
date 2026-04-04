"use client";

import { useState, useCallback, useRef } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { Send } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useConversationStore } from "@/lib/conversationStore";

const suggestions = [
  "Analyze the latest quarterly report",
  "Research competitive pricing strategies",
  "Review and summarize the contract",
  "Draft a project proposal",
];

export function EmptyState() {
  const router = useRouter();
  const { createConversation } = useConversationStore();
  const [input, setInput] = useState("");
  const [isCreating, setIsCreating] = useState(false);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  const handleSubmit = useCallback(
    async (text?: string) => {
      const content = (text ?? input).trim();
      if (!content || isCreating) return;

      setIsCreating(true);
      try {
        const conv = await createConversation();
        // Navigate to the conversation -- the ConversationView will handle
        // sending the first message via the input area after loading.
        // We encode the initial message as a query param.
        router.push(
          `/c/${conv.id}?q=${encodeURIComponent(content)}`,
        );
      } catch {
        setIsCreating(false);
      }
    },
    [input, isCreating, createConversation, router],
  );

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        void handleSubmit();
      }
    },
    [handleSubmit],
  );

  return (
    <div className="flex h-full flex-col items-center justify-center px-4">
      <div className="w-full max-w-2xl space-y-8 text-center">
        {/* Logo */}
        <div className="flex flex-col items-center gap-3">
          <Image src="/logo.svg" alt="TaskHub" width={48} height={48} />
          <h1 className="text-2xl font-semibold text-foreground">
            How can I help?
          </h1>
          <p className="text-sm text-muted-foreground">
            Start a conversation and our AI agents will collaborate to complete
            your task.
          </p>
        </div>

        {/* Input */}
        <div className="relative">
          <div className="flex items-end gap-2 rounded-xl border border-border bg-gray-900/50 p-3">
            <textarea
              ref={inputRef}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Describe your task..."
              rows={3}
              className="flex-1 resize-none bg-transparent text-sm text-foreground placeholder:text-muted-foreground outline-none"
            />
            <Button
              size="icon"
              onClick={() => void handleSubmit()}
              disabled={isCreating || !input.trim()}
            >
              <Send className="size-4" />
            </Button>
          </div>
        </div>

        {/* Suggestion buttons */}
        <div className="flex flex-wrap justify-center gap-2">
          {suggestions.map((s) => (
            <button
              key={s}
              onClick={() => void handleSubmit(s)}
              disabled={isCreating}
              className="rounded-full border border-border px-3 py-1.5 text-xs text-muted-foreground transition-colors hover:border-ring hover:text-foreground disabled:opacity-50"
            >
              {s}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
