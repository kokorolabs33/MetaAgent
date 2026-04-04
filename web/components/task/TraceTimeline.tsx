"use client";

import { useEffect, useState } from "react";
import { Loader2, ChevronDown, ChevronRight } from "lucide-react";
import { api } from "@/lib/api";
import type { TimelineEvent } from "@/lib/types";
import { cn } from "@/lib/utils";

interface TraceTimelineProps {
  taskId: string;
}

const categoryColors: Record<string, { dot: string; badge: string }> = {
  task: { dot: "bg-blue-500", badge: "bg-blue-500/20 text-blue-400" },
  subtask: { dot: "bg-green-500", badge: "bg-green-500/20 text-green-400" },
  message: { dot: "bg-gray-500", badge: "bg-gray-500/20 text-gray-400" },
  approval: { dot: "bg-amber-500", badge: "bg-amber-500/20 text-amber-400" },
  policy: { dot: "bg-purple-500", badge: "bg-purple-500/20 text-purple-400" },
  template: { dot: "bg-cyan-500", badge: "bg-cyan-500/20 text-cyan-400" },
  agent: { dot: "bg-red-500", badge: "bg-red-500/20 text-red-400" },
  replan: { dot: "bg-amber-500", badge: "bg-amber-500/20 text-amber-400" },
};

function getCategory(eventType: string): string {
  // Special case: task.replanned maps to replan category (amber), not task (blue)
  if (eventType === "task.replanned") return "replan";
  const prefix = eventType.split(".")[0];
  if (prefix in categoryColors) return prefix;
  return "message";
}

function getColors(eventType: string): { dot: string; badge: string } {
  const cat = getCategory(eventType);
  return categoryColors[cat] ?? categoryColors.message;
}

function formatRelativeTime(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const seconds = Math.floor(diff / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function formatDurationBetween(a: string, b: string): string {
  const diff = new Date(b).getTime() - new Date(a).getTime();
  if (diff < 0) return "";
  const ms = Math.abs(diff);
  if (ms < 1000) return `${ms}ms`;
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${minutes}m ${secs}s`;
}

function describeEvent(event: TimelineEvent): string {
  const data = event.data;
  switch (event.type) {
    case "task.created":
      return `Task created${data.title ? `: ${data.title}` : ""}`;
    case "task.started":
      return "Task execution started";
    case "task.completed":
      return "Task completed successfully";
    case "task.failed":
      return `Task failed${data.error ? `: ${data.error}` : ""}`;
    case "task.cancelled":
      return "Task was cancelled";
    case "subtask.created":
      return `Subtask created${data.instruction ? `: ${String(data.instruction).substring(0, 80)}` : ""}`;
    case "subtask.started":
      return `Subtask started${data.agent_name ? ` (agent: ${data.agent_name})` : ""}`;
    case "subtask.completed":
      return "Subtask completed";
    case "subtask.failed":
      return `Subtask failed${data.error ? `: ${data.error}` : ""}`;
    case "approval.requested":
      return "Approval requested";
    case "approval.granted":
      return "Approval granted";
    case "approval.rejected":
      return "Approval rejected";
    case "policy.evaluated":
      return `Policy evaluated: ${data.policy_name ?? "unknown"}`;
    case "task.replanned": {
      const replanData = event.data;
      const failedName = (replanData.failed_subtask as string) ?? "unknown";
      const newCount = (replanData.new_subtask_count as number) ?? 0;
      return `Replanned: subtask ${failedName} failed. ${newCount} new subtask${newCount !== 1 ? "s" : ""} created.`;
    }
    default:
      return event.type.replace(/[._]/g, " ");
  }
}

export function TraceTimeline({ taskId }: TraceTimelineProps) {
  const [events, setEvents] = useState<TimelineEvent[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());

  useEffect(() => {
    const load = async () => {
      try {
        setIsLoading(true);
        const result = await api.tasks.timeline(taskId);
        setEvents(result);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load timeline");
      } finally {
        setIsLoading(false);
      }
    };
    void load();
  }, [taskId]);

  const toggleExpand = (id: string) => {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center p-8">
        <Loader2 className="size-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex h-full items-center justify-center p-8 text-sm text-red-400">
        {error}
      </div>
    );
  }

  if (events.length === 0) {
    return (
      <div className="flex h-full items-center justify-center p-8 text-sm text-muted-foreground">
        No events recorded for this task
      </div>
    );
  }

  return (
    <div className="overflow-y-auto p-4">
      <div className="relative">
        {/* Vertical line */}
        <div className="absolute left-[11px] top-2 bottom-2 w-px bg-border" />

        {events.map((event, index) => {
          const colors = getColors(event.type);
          const isExpanded = expandedIds.has(event.id);
          const hasData =
            event.data && Object.keys(event.data).length > 0;
          const nextEvent = events[index + 1];
          const duration = nextEvent
            ? formatDurationBetween(event.created_at, nextEvent.created_at)
            : null;

          return (
            <div key={event.id} className="relative pb-1">
              {/* Event row */}
              <div className="flex items-start gap-3 pl-0">
                {/* Dot */}
                <div
                  className={cn(
                    "relative z-10 mt-1.5 size-[9px] shrink-0 rounded-full ring-2 ring-gray-950",
                    colors.dot,
                  )}
                />

                {/* Content */}
                <div className="min-w-0 flex-1 pb-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <span
                      className={cn(
                        "rounded px-1.5 py-0.5 text-[10px] font-medium",
                        colors.badge,
                      )}
                    >
                      {event.type}
                    </span>
                    <span className="text-[10px] text-muted-foreground">
                      {formatRelativeTime(event.created_at)}
                    </span>
                    {event.actor_type && event.actor_type !== "" && (
                      <span className="text-[10px] text-muted-foreground">
                        by {event.actor_type}
                      </span>
                    )}
                  </div>

                  <p className="mt-0.5 text-xs text-foreground">
                    {describeEvent(event)}
                  </p>

                  {/* Expandable data */}
                  {hasData && (
                    <button
                      onClick={() => toggleExpand(event.id)}
                      className="mt-1 flex items-center gap-1 text-[10px] text-muted-foreground hover:text-foreground"
                    >
                      {isExpanded ? (
                        <ChevronDown className="size-3" />
                      ) : (
                        <ChevronRight className="size-3" />
                      )}
                      {isExpanded ? "Hide" : "Show"} data
                    </button>
                  )}

                  {isExpanded && hasData && (
                    <pre className="mt-1 max-h-48 overflow-auto rounded bg-gray-900 p-2 text-[10px] text-muted-foreground">
                      {JSON.stringify(event.data, null, 2)}
                    </pre>
                  )}
                </div>
              </div>

              {/* Duration between events */}
              {duration && (
                <div className="relative ml-[7px] flex items-center py-0.5">
                  <span className="ml-5 text-[9px] text-muted-foreground/60">
                    {duration}
                  </span>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
