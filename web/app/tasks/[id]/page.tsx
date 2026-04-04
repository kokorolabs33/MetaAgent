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
  AlertCircle,
} from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { DAGView } from "@/components/task/DAGView";
import { TraceTimeline } from "@/components/task/TraceTimeline";
import { SubtaskDetailPanel } from "@/components/task/SubtaskDetailPanel";
import { GroupChat } from "@/components/chat/GroupChat";
import { useTaskStore, useAgentStore } from "@/lib/store";
import { useToast } from "@/components/ui/toast";
import { cn } from "@/lib/utils";
import type { Task, SubTask } from "@/lib/types";

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
  approval_required: {
    label: "Awaiting Approval",
    className: "bg-amber-500/20 text-amber-400",
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
  const [selectedSubtask, setSelectedSubtask] = useState<SubTask | null>(null);
  const [isApproving, setIsApproving] = useState(false);
  const [viewMode, setViewMode] = useState<"dag" | "timeline">("dag");
  const { addToast } = useToast();

  // Derive the latest version of the selected subtask from the store
  const activeSelectedSubtask = useMemo(() => {
    if (!selectedSubtask) return null;
    return (
      (currentTask?.subtasks ?? []).find((s) => s.id === selectedSubtask.id) ??
      selectedSubtask
    );
  }, [selectedSubtask, currentTask?.subtasks]);

  // Load agents
  useEffect(() => {
    if (allAgents.length === 0) {
      void loadAgents();
    }
  }, [allAgents.length, loadAgents]);

  // Load task + messages and connect SSE
  useEffect(() => {
    void selectTask(taskId);
    connectSSE(taskId);

    return () => {
      disconnectSSE();
    };
  }, [taskId, selectTask, connectSSE, disconnectSSE]);

  // Extract values for cost-loading effect dependency tracking
  const currentTaskId = currentTask?.id;
  const currentTaskStatus = currentTask?.status;

  // Load cost data
  useEffect(() => {
    if (!currentTaskId) return;

    const loadCost = async () => {
      try {
        const { api } = await import("@/lib/api");
        const cost = await api.tasks.cost(currentTaskId);
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
  }, [currentTaskId, currentTaskStatus]);

  const handleCancel = useCallback(async () => {
    if (isCancelling) return;
    setIsCancelling(true);
    try {
      await cancelTask(taskId);
      addToast("info", "Task cancelled");
    } catch {
      addToast("error", "Failed to cancel task");
    } finally {
      setIsCancelling(false);
    }
  }, [taskId, cancelTask, isCancelling, addToast]);

  const handleApproval = useCallback(
    async (action: "approve" | "reject") => {
      setIsApproving(true);
      try {
        const { api } = await import("@/lib/api");
        await api.tasks.approve(taskId, action);
        await selectTask(taskId);
        addToast(
          action === "approve" ? "success" : "info",
          action === "approve"
            ? "Task approved and execution started"
            : "Task rejected",
        );
      } catch {
        addToast("error", `Failed to ${action} task`);
      } finally {
        setIsApproving(false);
      }
    },
    [taskId, selectTask, addToast],
  );

  // Build agent list from subtasks — resolve names via agent store
  const taskAgents = useMemo(() => {
    const subtasks = currentTask?.subtasks ?? [];
    if (subtasks.length === 0) return [];
    const seen = new Set<string>();
    const result: { id: string; name: string }[] = [];
    for (const st of subtasks) {
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
          {/* View mode tabs */}
          <div className="flex border-b border-border">
            <button
              onClick={() => setViewMode("dag")}
              className={cn(
                "px-4 py-2 text-sm font-medium transition-colors",
                viewMode === "dag"
                  ? "border-b-2 border-blue-500 text-blue-400"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              DAG
            </button>
            <button
              onClick={() => setViewMode("timeline")}
              className={cn(
                "px-4 py-2 text-sm font-medium transition-colors",
                viewMode === "timeline"
                  ? "border-b-2 border-blue-500 text-blue-400"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              Timeline
            </button>
          </div>

          {viewMode === "dag" ? (
            <>
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
                  subtasks={currentTask.subtasks ?? []}
                  agentNames={Object.fromEntries(allAgents.map((a) => [a.id, a.name]))}
                  onNodeClick={(subtaskId) => {
                    const st = (currentTask.subtasks ?? []).find(
                      (s) => s.id === subtaskId,
                    );
                    if (st) setSelectedSubtask(st);
                  }}
                />
              </div>
            </>
          ) : (
            <div className="flex-1 overflow-y-auto">
              <TraceTimeline taskId={taskId} />
            </div>
          )}
        </div>

        {/* Group chat (right 60%) */}
        <div className="flex-1">
          <GroupChat
            taskId={taskId}
            messages={messages}
            agents={taskAgents}
            subtasks={currentTask.subtasks ?? []}
          />
        </div>
      </div>

      {/* Subtask detail panel */}
      {activeSelectedSubtask && (
        <SubtaskDetailPanel
          subtask={activeSelectedSubtask}
          agentName={
            allAgents.find((a) => a.id === activeSelectedSubtask.agent_id)
              ?.name ?? activeSelectedSubtask.agent_id
          }
          onClose={() => setSelectedSubtask(null)}
        />
      )}

      {/* Approval required banner */}
      {currentTask.status === "approval_required" && (
        <div className="border-t border-amber-500/30 bg-amber-950/20 px-4 py-3">
          <div className="flex items-center justify-between">
            <div>
              <div className="flex items-center gap-2 text-sm font-semibold text-amber-400">
                <AlertCircle className="size-4" />
                Approval Required
              </div>
              <p className="mt-1 text-xs text-amber-500/70">
                The execution plan has {(currentTask.subtasks ?? []).length} subtasks
                and requires your approval to proceed.
              </p>
            </div>
            <div className="flex gap-2">
              <Button
                variant="destructive"
                size="sm"
                onClick={() => void handleApproval("reject")}
                disabled={isApproving}
              >
                Reject
              </Button>
              <Button
                size="sm"
                className="bg-green-600 text-white hover:bg-green-500"
                onClick={() => void handleApproval("approve")}
                disabled={isApproving}
              >
                {isApproving ? "Approving..." : "Approve & Execute"}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Task completed banner */}
      {currentTask.status === "completed" && (
        <div className="border-t border-green-500/30 bg-green-950/20 px-4 py-2">
          <div className="flex items-center gap-2 text-sm text-green-400">
            <span className="font-semibold">Task completed</span>
            <span className="text-xs text-green-500/70">
              — {(currentTask.subtasks ?? []).filter(st => st.status === "completed").length}/{(currentTask.subtasks ?? []).length} subtasks finished
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
            Subtasks: {(currentTask.subtasks ?? []).filter((s) => s.status === "completed").length}/
            {(currentTask.subtasks ?? []).length}
          </span>
        </div>
      </div>
    </div>
  );
}
