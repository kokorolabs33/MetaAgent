"use client";

interface TypingIndicatorProps {
  agentName: string;
}

export function TypingIndicator({ agentName }: TypingIndicatorProps) {
  return (
    <div className="flex items-center gap-2 px-4 py-2">
      <div className="flex gap-1">
        <span className="size-1.5 animate-bounce rounded-full bg-blue-400 [animation-delay:0ms]" />
        <span className="size-1.5 animate-bounce rounded-full bg-blue-400 [animation-delay:150ms]" />
        <span className="size-1.5 animate-bounce rounded-full bg-blue-400 [animation-delay:300ms]" />
      </div>
      <span className="text-xs text-muted-foreground">
        {agentName} is typing...
      </span>
    </div>
  );
}
