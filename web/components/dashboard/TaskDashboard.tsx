"use client";

import { useEffect, useMemo } from "react";
import { useRouter, useSearchParams, usePathname } from "next/navigation";
import { Loader2, AlertCircle } from "lucide-react";
import { useDashboardStore, useAgentStore } from "@/lib/store";
import { TaskFilterBar } from "@/components/dashboard/TaskFilterBar";
import { DashboardTaskCard } from "@/components/dashboard/DashboardTaskCard";
import {
  DashboardEmptyState,
  type DashboardEmptyStateVariant,
} from "@/components/dashboard/DashboardEmptyState";
import { Pagination } from "@/components/ui/pagination";

export function TaskDashboard() {
  const searchParams = useSearchParams();
  const pathname = usePathname();
  const router = useRouter();

  const status = searchParams.get("status") ?? undefined;
  const q = searchParams.get("q") ?? undefined;
  const page = Math.max(1, Number(searchParams.get("page") ?? "1"));

  const tasks = useDashboardStore((s) => s.tasks);
  const totalPages = useDashboardStore((s) => s.totalPages);
  const currentPage = useDashboardStore((s) => s.currentPage);
  const isLoading = useDashboardStore((s) => s.isLoading);
  const error = useDashboardStore((s) => s.error);
  const loadDashboard = useDashboardStore((s) => s.loadDashboard);
  const connectDashboardSSE = useDashboardStore((s) => s.connectDashboardSSE);
  const disconnectDashboardSSE = useDashboardStore((s) => s.disconnectDashboardSSE);
  const taskProgress = useDashboardStore((s) => s.taskProgress);

  const connectStatusSSE = useAgentStore((s) => s.connectStatusSSE);
  const disconnectStatusSSE = useAgentStore((s) => s.disconnectStatusSSE);
  const getAgentStatus = useAgentStore((s) => s.getAgentStatus);

  // Load on filter change. useEffect dep is the primitive strings, not
  // objects, so changes to the store itself do not re-trigger the load.
  useEffect(() => {
    void loadDashboard({ status, q, page });
  }, [status, q, page, loadDashboard]);

  // Connect multiplexed task SSE keyed to the visible task IDs.
  // Use ids.join(",") as the effect dep (Pitfall 4 in research) so a new
  // array reference with the same ids does NOT tear down the connection.
  const idsKey = useMemo(() => tasks.map((t) => t.id).join(","), [tasks]);
  useEffect(() => {
    if (idsKey === "") {
      disconnectDashboardSSE();
      return;
    }
    connectDashboardSSE(idsKey.split(","));
    return () => disconnectDashboardSSE();
  }, [idsKey, connectDashboardSSE, disconnectDashboardSSE]);

  // Connect global agent status stream for dots.
  useEffect(() => {
    connectStatusSSE();
    return () => disconnectStatusSSE();
  }, [connectStatusSSE, disconnectStatusSSE]);

  const handlePageChange = (nextPage: number) => {
    const params = new URLSearchParams(searchParams.toString());
    if (nextPage <= 1) {
      params.delete("page");
    } else {
      params.set("page", String(nextPage));
    }
    const qs = params.toString();
    router.replace(qs ? `${pathname}?${qs}` : pathname);
  };

  // Determine empty-state variant.
  const emptyVariant: DashboardEmptyStateVariant = q
    ? "search"
    : status === "running"
      ? "running"
      : status === "completed"
        ? "completed"
        : status === "failed"
          ? "failed"
          : "all";

  // Agent IDs per card come directly from the Task interface -- Plan 01
  // extended GET /api/tasks to return `agent_ids: string[]` per task via
  // `ARRAY_AGG(DISTINCT s.agent_id) FILTER (WHERE s.agent_id IS NOT NULL)`
  // in the List query. This delivers CONTEXT.md D-02 in full: each card
  // renders the Phase 2 AgentStatusDot row using the agents actually
  // assigned to that task's subtasks. The backend guarantees agent_ids
  // is always an array (empty for tasks with no subtasks), never null.
  const agentIdsForTask = (taskId: string): string[] => {
    const task = tasks.find((t) => t.id === taskId);
    return task?.agent_ids ?? [];
  };

  return (
    <div
      className="flex h-full flex-col"
      aria-live="polite"
    >
      <TaskFilterBar />

      <div className="flex-1 overflow-auto px-6 py-6">
        {error ? (
          <div className="flex h-full flex-col items-center justify-center px-4 text-center">
            <AlertCircle
              className="mb-4 size-10 text-destructive"
              aria-hidden="true"
            />
            <p className="text-sm text-muted-foreground">{error}</p>
          </div>
        ) : isLoading && tasks.length === 0 ? (
          <div className="flex h-full items-center justify-center">
            <Loader2
              className="size-6 animate-spin text-muted-foreground"
              aria-label="Loading tasks"
            />
          </div>
        ) : tasks.length === 0 ? (
          <DashboardEmptyState variant={emptyVariant} />
        ) : (
          <>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {tasks.map((task) => (
                <DashboardTaskCard
                  key={task.id}
                  task={task}
                  progress={taskProgress[task.id]}
                  agentIds={agentIdsForTask(task.id)}
                  getAgentStatus={getAgentStatus}
                />
              ))}
            </div>
            {totalPages > 1 && (
              <div className="mt-6 flex justify-center">
                <Pagination
                  currentPage={currentPage}
                  totalPages={totalPages}
                  onPageChange={handlePageChange}
                />
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
