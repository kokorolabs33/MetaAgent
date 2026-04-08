"use client";

import { useCallback } from "react";
import { useRouter } from "next/navigation";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { AgentStatusDot } from "@/components/agent/AgentStatusDot";
import { SubtaskProgressBar } from "@/components/dashboard/SubtaskProgressBar";
import type { Task, AgentActivityStatus } from "@/lib/types";

// Copied verbatim from web/components/dashboard/TaskCard.tsx:8-40 — UI-SPEC
// mandates exact reuse of the existing statusConfig so the two cards stay
// in visual lockstep. If TaskCard's statusConfig changes, mirror it here.
const statusConfig: Record<
  Task["status"],
  { label: string; className: string }
> = {
  pending: {
    label: "Pending",
    className: "bg-gray-500/20 text-gray-400",
  },
  planning: {
    label: "Planning",
    className: "bg-blue-500/20 text-blue-400",
  },
  running: {
    label: "Running",
    className: "bg-amber-500/20 text-amber-400",
  },
  completed: {
    label: "Completed",
    className: "bg-green-500/20 text-green-400",
  },
  failed: {
    label: "Failed",
    className: "bg-red-500/20 text-red-400",
  },
  cancelled: {
    label: "Cancelled",
    className: "bg-gray-500/20 text-gray-500 line-through",
  },
  approval_required: {
    label: "Awaiting Approval",
    className: "bg-amber-500/20 text-amber-400",
  },
};

// Copied verbatim from web/components/dashboard/TaskCard.tsx:42-55.
function timeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diffMs = now - then;
  const diffSec = Math.floor(diffMs / 1000);

  if (diffSec < 60) return `${diffSec}s ago`;
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  const diffDay = Math.floor(diffHr / 24);
  return `${diffDay}d ago`;
}

const MAX_VISIBLE_DOTS = 5;

interface DashboardTaskCardProps {
  task: Task;
  /** Subtask counts from SSE-updated dashboard store (falls back to task fields). */
  progress?: { completed: number; total: number };
  /** Agent IDs to render as status dots (e.g. the unique agents across the task's subtasks). */
  agentIds: string[];
  /** Resolver for agent activity status; typically useAgentStore.getAgentStatus. */
  getAgentStatus: (agentId: string) => AgentActivityStatus;
}

export function DashboardTaskCard({
  task,
  progress,
  agentIds,
  getAgentStatus,
}: DashboardTaskCardProps) {
  const router = useRouter();
  const config = statusConfig[task.status];

  // Prefer the live progress map (SSE-updated) but fall back to the task
  // fields returned by the list API so the card is still meaningful before
  // SSE delivers any deltas.
  const completed = progress?.completed ?? task.completed_subtasks;
  const total = progress?.total ?? task.total_subtasks;
  const isFailed = task.status === "failed";

  const visibleAgents = agentIds.slice(0, MAX_VISIBLE_DOTS);
  const overflow = agentIds.length - visibleAgents.length;

  const handleClick = useCallback(() => {
    router.push(`/tasks/${task.id}`);
  }, [router, task.id]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLDivElement>) => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        handleClick();
      }
    },
    [handleClick],
  );

  return (
    <Card
      role="article"
      aria-label={`${task.title} - ${config.label}`}
      tabIndex={0}
      className="cursor-pointer outline-none transition-colors hover:bg-card/80 focus-visible:ring-2 focus-visible:ring-primary"
      onClick={handleClick}
      onKeyDown={handleKeyDown}
    >
      <CardHeader>
        <div className="flex items-center justify-between gap-2">
          <CardTitle className="truncate text-base font-bold">
            {task.title}
          </CardTitle>
          <Badge className={config.className}>{config.label}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        <div className="flex flex-col gap-3">
          <SubtaskProgressBar
            completed={completed}
            total={total}
            failed={isFailed}
          />
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-1">
              {visibleAgents.map((agentId) => (
                <AgentStatusDot
                  key={agentId}
                  status={getAgentStatus(agentId)}
                  size="sm"
                />
              ))}
              {overflow > 0 && (
                <span className="text-xs text-muted-foreground">
                  +{overflow}
                </span>
              )}
            </div>
            <span className="shrink-0 text-xs text-muted-foreground">
              {timeAgo(task.created_at)}
            </span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
