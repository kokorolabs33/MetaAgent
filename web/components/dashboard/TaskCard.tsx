"use client";

import { useRouter } from "next/navigation";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { Task } from "@/lib/types";

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

interface TaskCardProps {
  task: Task;
  hasWaitingSubtask?: boolean;
}

export function TaskCard({ task, hasWaitingSubtask }: TaskCardProps) {
  const router = useRouter();
  const config = statusConfig[task.status];

  return (
    <Card
      className="cursor-pointer transition-colors hover:bg-card/80"
      onClick={() => router.push(`/tasks/${task.id}`)}
    >
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <CardTitle className="truncate">{task.title}</CardTitle>
            {hasWaitingSubtask && (
              <span className="relative flex size-2.5">
                <span className="absolute inline-flex size-full animate-ping rounded-full bg-red-400 opacity-75" />
                <span className="relative inline-flex size-2.5 rounded-full bg-red-500" />
              </span>
            )}
          </div>
          <Badge className={config.className}>{config.label}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        <div className="flex items-center justify-between">
          {task.description ? (
            <p className="truncate text-xs text-muted-foreground">
              {task.description}
            </p>
          ) : (
            <span />
          )}
          <span className="shrink-0 text-xs text-muted-foreground">
            {timeAgo(task.created_at)}
          </span>
        </div>
      </CardContent>
    </Card>
  );
}
