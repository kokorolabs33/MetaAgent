"use client";

import { Search, CheckCircle2, Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";
import type { ToolCallEvent } from "@/lib/types";

interface ToolCallStatusProps {
  event: ToolCallEvent;
}

const toolIcons: Record<string, React.ComponentType<{ className?: string }>> = {
  web_search: Search,
};

function formatToolDisplay(toolName: string, args?: string): string {
  if (toolName === "web_search" && args) {
    try {
      const parsed = JSON.parse(args) as { query?: string };
      if (parsed.query) return `Searching: "${parsed.query}"`;
    } catch {
      // fall through
    }
  }
  return `Running: ${toolName.replace(/_/g, " ")}`;
}

export function ToolCallStatus({ event }: ToolCallStatusProps) {
  const Icon = toolIcons[event.tool_name] ?? Search;
  const isActive = event.status === "started";

  return (
    <div
      className={cn(
        "flex items-center gap-2 px-4 py-1.5 text-xs",
        isActive ? "text-blue-400" : "text-gray-500",
      )}
    >
      <div className="flex size-5 items-center justify-center">
        {isActive ? (
          <Loader2 className="size-3.5 animate-spin" />
        ) : (
          <CheckCircle2 className="size-3.5" />
        )}
      </div>
      <Icon className="size-3.5" />
      <span>
        {isActive
          ? formatToolDisplay(event.tool_name, event.args)
          : `${event.tool_name.replace(/_/g, " ")} complete`}
      </span>
    </div>
  );
}
