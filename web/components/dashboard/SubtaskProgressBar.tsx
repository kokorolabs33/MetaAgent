"use client";

import { cn } from "@/lib/utils";

interface SubtaskProgressBarProps {
  completed: number;
  total: number;
  /** When true, renders the bar in destructive red regardless of percentage. */
  failed?: boolean;
}

export function SubtaskProgressBar({ completed, total, failed }: SubtaskProgressBarProps) {
  const safeTotal = Math.max(total, 0);
  const safeCompleted = Math.max(0, Math.min(completed, safeTotal));
  const pct = safeTotal > 0 ? (safeCompleted / safeTotal) * 100 : 0;

  const barColor = failed
    ? "bg-red-500"
    : pct >= 100
      ? "bg-green-500"
      : pct >= 50
        ? "bg-blue-500"
        : "bg-amber-500";

  return (
    <div className="flex items-center gap-2">
      <div
        className="h-1.5 flex-1 overflow-hidden rounded-full bg-gray-800"
        role="progressbar"
        aria-valuenow={safeCompleted}
        aria-valuemin={0}
        aria-valuemax={safeTotal}
        aria-label="Subtask progress"
      >
        <div
          className={cn("h-full rounded-full transition-all duration-300", barColor)}
          style={{ width: `${pct}%` }}
        />
      </div>
      <span className="shrink-0 text-xs text-muted-foreground">
        {safeCompleted}/{safeTotal} subtasks
      </span>
    </div>
  );
}
