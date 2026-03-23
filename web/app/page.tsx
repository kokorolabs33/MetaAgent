"use client";

import { useEffect, useMemo, useState, useCallback } from "react";
import { useOrgStore, useTaskStore } from "@/lib/store";
import { TaskCard } from "@/components/dashboard/TaskCard";
import { NewTaskDialog } from "@/components/dashboard/NewTaskDialog";
import type { Task } from "@/lib/types";

const statusFilters: { label: string; value: string }[] = [
  { label: "All", value: "" },
  { label: "Pending", value: "pending" },
  { label: "Planning", value: "planning" },
  { label: "Running", value: "running" },
  { label: "Completed", value: "completed" },
  { label: "Failed", value: "failed" },
  { label: "Cancelled", value: "cancelled" },
];

export default function DashboardPage() {
  const { loadOrgs, orgs, isLoading: orgsLoading } = useOrgStore();
  const {
    tasks,
    loadTasks,
    isLoading: tasksLoading,
  } = useTaskStore();
  const [statusFilter, setStatusFilter] = useState("");

  // Derive orgId from loaded orgs (use first org)
  const orgId = useMemo(
    () => (orgs.length > 0 ? orgs[0].id : null),
    [orgs],
  );

  // Load orgs on mount
  useEffect(() => {
    loadOrgs();
  }, [loadOrgs]);

  // Load tasks when org is selected or filter changes
  const loadFilteredTasks = useCallback(() => {
    if (orgId) {
      loadTasks(orgId, statusFilter || undefined);
    }
  }, [orgId, statusFilter, loadTasks]);

  useEffect(() => {
    loadFilteredTasks();
  }, [loadFilteredTasks]);

  const isLoading = orgsLoading || tasksLoading;

  // No org available yet
  if (!orgsLoading && orgs.length === 0) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-center">
          <h2 className="text-lg font-semibold text-foreground">
            Welcome to TaskHub
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            No organisations found. Create one to get started.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-border px-6 py-4">
        <div className="flex items-center gap-4">
          <h1 className="text-lg font-semibold text-foreground">Tasks</h1>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm text-foreground transition-colors focus-visible:border-ring focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
          >
            {statusFilters.map((f) => (
              <option key={f.value} value={f.value} className="bg-card">
                {f.label}
              </option>
            ))}
          </select>
        </div>
        {orgId && <NewTaskDialog orgId={orgId} />}
      </div>

      {/* Task list */}
      <div className="flex-1 overflow-auto p-6">
        {isLoading ? (
          <div className="flex h-64 items-center justify-center">
            <p className="text-sm text-muted-foreground">Loading tasks...</p>
          </div>
        ) : tasks.length === 0 ? (
          <div className="flex h-64 items-center justify-center">
            <div className="text-center">
              <p className="text-sm text-muted-foreground">
                No tasks yet. Create your first task!
              </p>
            </div>
          </div>
        ) : (
          <div className="grid gap-3">
            {tasks.map((task: Task) => (
              <TaskCard key={task.id} task={task} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
