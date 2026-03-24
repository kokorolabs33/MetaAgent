"use client";

import { useEffect, useMemo, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import {
  ArrowLeft,
  Loader2,
  Clock,
  DollarSign,
  Hash,
  XCircle,
} from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { DAGView } from "@/components/task/DAGView";
import { GroupChat } from "@/components/chat/GroupChat";
import { useOrgStore, useTaskStore, useAgentStore } from "@/lib/store";
import type { Task } from "@/lib/types";

const statusConfig: Record<
  Task["status"],
  { label: string; className: string }
> = {
  pending: { label: "Pending", className: "bg-gray-500/20 text-gray-400" },
  planning: { label: "Planning", className: "bg-blue-500/20 text-blue-400" },
  running: { label: "Running", className: "bg-amber-500/20 text-amber-400" },
  completed: {
    label: "Completed",
    className: "bg-green-500/20 text-green-400",
  },
  failed: { label: "Failed", className: "bg-red-500/20 text-red-400" },
  cancelled: {
    label: "Cancelled",
    className: "bg-gray-500/20 text-gray-500",
  },
};

function formatDuration(createdAt: string): string {
  const diffMs = Date.now() - new Date(createdAt).getTime();
  const totalSec = Math.floor(diffMs / 1000);

  if (totalSec < 60) return `${totalSec}s`;
  const min = Math.floor(totalSec / 60);
  const sec = totalSec % 60;
  if (min < 60) return `${min}m ${sec}s`;
  const hr = Math.floor(min / 60);
  const remMin = min % 60;
  return `${hr}h ${remMin}m`;
}

export default function TaskDetailPage() {
  const params = useParams();
  const taskId = params.id as string;

  const { orgs, loadOrgs } = useOrgStore();
  const {
    currentTask,
    messages,
    isLoading,
    selectTask,
    cancelTask,
    connectSSE,
    disconnectSSE,
  } = useTaskStore();
  const { agents: allAgents, loadAgents } = useAgentStore();

  const [costData, setCostData] = useState<{
    total_cost_usd: number;
    total_input_tokens: number;
    total_output_tokens: number;
  } | null>(null);
  const [isCancelling, setIsCancelling] = useState(false);

  const orgId = useMemo(
    () => (orgs.length > 0 ? orgs[0].id : null),
    [orgs],
  );

  // Load orgs if needed
  useEffect(() => {
    if (orgs.length === 0) {
      loadOrgs();
    }
  }, [orgs.length, loadOrgs]);

  // Load agents for this org
  useEffect(() => {
    if (orgId && allAgents.length === 0) {
      void loadAgents(orgId);
    }
  }, [orgId, allAgents.length, loadAgents]);

  // Load task + messages and connect SSE
  useEffect(() => {
    if (!orgId) return;

    void selectTask(orgId, taskId);
    connectSSE(orgId, taskId);

    return () => {
      disconnectSSE();
    };
  }, [orgId, taskId, selectTask, connectSSE, disconnectSSE]);

  // Extract values for cost-loading effect dependency tracking
  const currentTaskId = currentTask?.id;
  const currentTaskStatus = currentTask?.status;

  // Load cost data
  useEffect(() => {
    if (!orgId || !currentTaskId) return;

    const loadCost = async () => {
      try {
        const { api } = await import("@/lib/api");
        const cost = await api.tasks.cost(orgId, currentTaskId);
        setCostData(cost);
      } catch {
        // Cost data may not be available yet
      }
    };

    void loadCost();

    // Refresh cost data periodically while task is active
    const taskIsActive =
      currentTaskStatus === "running" || currentTaskStatus === "planning";
    if (!taskIsActive) return;

    const interval = setInterval(() => void loadCost(), 15000);
    return () => clearInterval(interval);
  }, [orgId, currentTaskId, currentTaskStatus]);

  const handleCancel = useCallback(async () => {
    if (!orgId || isCancelling) return;
    setIsCancelling(true);
    try {
      await cancelTask(orgId, taskId);
    } catch {
      // Error managed by store
    } finally {
      setIsCancelling(false);
    }
  }, [orgId, taskId, cancelTask, isCancelling]);

  // Build agent list from subtasks — resolve names via agent store
  const taskAgents = useMemo(() => {
    if (!currentTask?.subtasks) return [];
    const seen = new Set<string>();
    const result: { id: string; name: string }[] = [];
    for (const st of currentTask.subtasks) {
      if (seen.has(st.agent_id)) continue;
      seen.add(st.agent_id);
      const agent = allAgents.find((a) => a.id === st.agent_id);
      result.push({ id: st.agent_id, name: agent?.name ?? st.agent_id });
    }
    return result;
  }, [currentTask?.subtasks, allAgents]);

  // Loading state
  if (isLoading || !currentTask) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  const config = statusConfig[currentTask.status];
  const isActive =
    currentTask.status === "running" || currentTask.status === "planning";

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <div className="flex items-center gap-3">
          <Link
            href="/"
            className="rounded-md p-1 text-muted-foreground transition-colors hover:text-foreground"
          >
            <ArrowLeft className="size-5" />
          </Link>
          <h1 className="truncate text-lg font-semibold text-foreground">
            {currentTask.title}
          </h1>
          <Badge className={config.className}>{config.label}</Badge>
        </div>
        <div className="flex items-center gap-2">
          {isActive && (
            <Button
              variant="destructive"
              size="sm"
              onClick={() => void handleCancel()}
              disabled={isCancelling}
            >
              <XCircle className="size-4" />
              {isCancelling ? "Cancelling..." : "Cancel"}
            </Button>
          )}
        </div>
      </div>

      {/* Main content: DAG + Chat split */}
      <div className="flex flex-1 overflow-hidden">
        {/* DAG view (left 40%) */}
        <div className="flex w-2/5 flex-col border-r border-border">
          {/* Participating agents header */}
          {taskAgents.length > 0 && (
            <div className="border-b border-border px-3 py-2">
              <div className="mb-1.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                Agents ({taskAgents.length})
              </div>
              <div className="flex flex-wrap gap-1.5">
                {taskAgents.map((a) => (
                  <div
                    key={a.id}
                    className="flex items-center gap-1.5 rounded-full bg-blue-600/20 px-2.5 py-0.5 text-xs text-blue-300"
                  >
                    <div className="size-2 rounded-full bg-blue-500" />
                    {a.name}
                  </div>
                ))}
              </div>
            </div>
          )}
          <div className="flex-1">
            <DAGView
              subtasks={currentTask.subtasks}
              agentNames={Object.fromEntries(allAgents.map((a) => [a.id, a.name]))}
              onNodeClick={(subtaskId) => {
                const _id = subtaskId;
                void _id;
              }}
            />
          </div>
        </div>

        {/* Group chat (right 60%) */}
        <div className="flex-1">
          {orgId && (
            <GroupChat
              orgId={orgId}
              taskId={taskId}
              messages={messages}
              agents={taskAgents}
              subtasks={currentTask.subtasks}
            />
          )}
        </div>
      </div>

      {/* Task completed banner */}
      {currentTask.status === "completed" && (
        <div className="border-t border-green-500/30 bg-green-950/20 px-4 py-2">
          <div className="flex items-center gap-2 text-sm text-green-400">
            <span className="font-semibold">Task completed</span>
            <span className="text-xs text-green-500/70">
              — {currentTask.subtasks.filter(st => st.status === "completed").length}/{currentTask.subtasks.length} subtasks finished
            </span>
          </div>
        </div>
      )}

      {/* Bottom status bar */}
      <div className="flex items-center gap-6 border-t border-border bg-gray-900/50 px-4 py-2 text-xs text-muted-foreground">
        <div className="flex items-center gap-1.5">
          <Clock className="size-3.5" />
          <span>{formatDuration(currentTask.created_at)}</span>
        </div>
        {costData && (
          <>
            <div className="flex items-center gap-1.5">
              <DollarSign className="size-3.5" />
              <span>${costData.total_cost_usd.toFixed(4)}</span>
            </div>
            <div className="flex items-center gap-1.5">
              <Hash className="size-3.5" />
              <span>
                {(
                  costData.total_input_tokens + costData.total_output_tokens
                ).toLocaleString()}{" "}
                tokens
              </span>
            </div>
          </>
        )}
        <div className="flex items-center gap-1.5">
          <span>
            Subtasks: {currentTask.subtasks.filter((s) => s.status === "completed").length}/
            {currentTask.subtasks.length}
          </span>
        </div>
      </div>
    </div>
  );
}
