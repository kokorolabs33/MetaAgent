"use client";

import { Inbox } from "lucide-react";

export type DashboardEmptyStateVariant = "all" | "running" | "completed" | "failed" | "search";

interface DashboardEmptyStateProps {
  variant: DashboardEmptyStateVariant;
}

const copy: Record<DashboardEmptyStateVariant, { heading: string; body: string }> = {
  all: {
    heading: "No tasks yet",
    body: "Create your first task and watch AI agents collaborate in real time.",
  },
  running: {
    heading: "No running tasks",
    body: "Start a new task or check the All tab for existing work.",
  },
  completed: {
    heading: "No completed tasks",
    body: "Tasks will appear here once agents finish their work.",
  },
  failed: {
    heading: "No failed tasks",
    body: "All clear. No tasks have failed.",
  },
  search: {
    heading: "No matching tasks",
    body: "Try a different search term or clear the filter.",
  },
};

export function DashboardEmptyState({ variant }: DashboardEmptyStateProps) {
  const { heading, body } = copy[variant];
  return (
    <div className="flex h-full flex-col items-center justify-center px-4 py-16 text-center">
      <Inbox className="mb-4 size-10 text-muted-foreground" aria-hidden="true" />
      <h3 className="text-base font-bold text-foreground">{heading}</h3>
      <p className="mt-2 max-w-sm text-sm text-muted-foreground">{body}</p>
    </div>
  );
}
