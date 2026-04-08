"use client";

import { useState, useMemo, useEffect, useCallback } from "react";
import {
  ChevronDown,
  ExternalLink,
  Clock,
  Loader2,
  CheckCircle2,
  XCircle,
  AlertCircle,
  Lock,
  Ban,
  Pause,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api";
import { useConversationStore } from "@/lib/conversationStore";
import { SubtaskDetailPanel } from "@/components/task/SubtaskDetailPanel";
import { useAgentStore } from "@/lib/store";
import type { Task, SubTask, TaskWithSubtasks, ExecutionPlan, PlanSubTask } from "@/lib/types";

const taskStatusConfig: Record<
  Task["status"],
  { label: string; className: string }
> = {
  pending: { label: "Pending", className: "text-gray-400" },
  planning: { label: "Planning", className: "text-blue-400" },
  running: { label: "Running", className: "text-amber-400" },
  completed: { label: "Completed", className: "text-green-400" },
  failed: { label: "Failed", className: "text-red-400" },
  cancelled: { label: "Cancelled", className: "text-gray-500" },
  approval_required: { label: "Approval", className: "text-amber-400" },
};

const subtaskIcons: Record<string, React.ElementType> = {
  pending: Clock,
  running: Loader2,
  completed: CheckCircle2,
  failed: XCircle,
  input_required: AlertCircle,
  approval_required: Pause,
  blocked: Lock,
  cancelled: Ban,
};

const subtaskColors: Record<string, string> = {
  pending: "text-gray-400",
  running: "text-blue-400",
  completed: "text-green-400",
  failed: "text-red-400",
  input_required: "text-amber-400",
  approval_required: "text-amber-400",
  blocked: "text-orange-400",
  cancelled: "text-gray-500",
};

function groupIntoWaves(
  subtasks: SubTask[],
): { wave: number; items: SubTask[] }[] {
  const waves: Map<number, SubTask[]> = new Map();
  const subtaskWave: Map<string, number> = new Map();

  function getWave(st: SubTask): number {
    if (subtaskWave.has(st.id)) return subtaskWave.get(st.id)!;
    if (!st.depends_on?.length) {
      subtaskWave.set(st.id, 0);
      return 0;
    }
    const depWaves = st.depends_on.map((depId) => {
      const dep = subtasks.find((s) => s.id === depId);
      return dep ? getWave(dep) : 0;
    });
    const w = Math.max(...depWaves) + 1;
    subtaskWave.set(st.id, w);
    return w;
  }

  for (const st of subtasks) {
    const w = getWave(st);
    if (!waves.has(w)) waves.set(w, []);
    waves.get(w)!.push(st);
  }

  return Array.from(waves.entries())
    .sort(([a], [b]) => a - b)
    .map(([wave, items]) => ({ wave, items }));
}

export function DAGPanel() {
  const { tasks } = useConversationStore();
  const { agents: allAgents, loadAgents } = useAgentStore();
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const [showDropdown, setShowDropdown] = useState(false);
  const [taskDetail, setTaskDetail] = useState<TaskWithSubtasks | null>(null);
  const [selectedSubtask, setSelectedSubtask] = useState<SubTask | null>(null);
  const [isLoadingDetail, setIsLoadingDetail] = useState(false);

  // Load agents if needed
  useEffect(() => {
    if (allAgents.length === 0) {
      void loadAgents();
    }
  }, [allAgents.length, loadAgents]);

  // Auto-select first task or most recent active
  useEffect(() => {
    if (tasks.length === 0) {
      setSelectedTaskId(null);
      setTaskDetail(null);
      return;
    }
    const active = tasks.find(
      (t) => t.status === "running" || t.status === "planning" || t.status === "approval_required",
    );
    const target = active ?? tasks[tasks.length - 1];
    if (target && target.id !== selectedTaskId) {
      setSelectedTaskId(target.id);
    }
  }, [tasks, selectedTaskId]);

  // Load task detail when selected task changes
  const loadDetail = useCallback(async (taskId: string) => {
    setIsLoadingDetail(true);
    try {
      const detail = await api.tasks.get(taskId);
      setTaskDetail(detail);
    } catch {
      setTaskDetail(null);
    } finally {
      setIsLoadingDetail(false);
    }
  }, []);

  useEffect(() => {
    if (selectedTaskId) {
      void loadDetail(selectedTaskId);
    }
  }, [selectedTaskId, loadDetail]);

  // Periodically refresh subtask status for active (non-terminal) tasks
  const terminalStatuses = new Set(["completed", "failed", "cancelled"]);
  useEffect(() => {
    if (!selectedTaskId) return;
    const selectedTask = tasks.find((t) => t.id === selectedTaskId);
    if (!selectedTask || terminalStatuses.has(selectedTask.status)) return;

    const interval = setInterval(() => {
      void loadDetail(selectedTaskId);
    }, 5000);
    return () => clearInterval(interval);
  }, [selectedTaskId, tasks, loadDetail]);

  // Re-fetch task detail when the store's task status changes (via SSE events)
  const storeTaskStatus = tasks.find((t) => t.id === selectedTaskId)?.status;
  useEffect(() => {
    if (selectedTaskId && storeTaskStatus) {
      void loadDetail(selectedTaskId);
    }
  }, [selectedTaskId, storeTaskStatus, loadDetail]);

  const subtasks = useMemo(() => taskDetail?.subtasks ?? [], [taskDetail]);
  const waves = useMemo(() => groupIntoWaves(subtasks), [subtasks]);
  const selectedTask = tasks.find((t) => t.id === selectedTaskId);

  // Parse plan preview for approval_required tasks (subtasks not yet in DB)
  const planPreview = useMemo((): ExecutionPlan | null => {
    if (!selectedTask || selectedTask.status !== "approval_required") return null;
    if (subtasks.length > 0) return null;
    const plan = selectedTask.plan as unknown as ExecutionPlan | undefined;
    if (!plan || !Array.isArray(plan.subtasks)) return null;
    return plan;
  }, [selectedTask, subtasks]);

  const planWaves = useMemo(() => {
    if (!planPreview) return [];
    const fakeSubtasks: SubTask[] = planPreview.subtasks.map((ps) => ({
      id: ps.id,
      task_id: "",
      agent_id: ps.agent_id,
      instruction: ps.instruction,
      depends_on: ps.depends_on ?? [],
      status: "pending" as const,
      attempt: 0,
      max_attempts: 3,
      created_at: "",
    }));
    return groupIntoWaves(fakeSubtasks);
  }, [planPreview]);

  const completedCount = subtasks.filter(
    (s) => s.status === "completed",
  ).length;
  const total = subtasks.length;
  const pct = total > 0 ? Math.round((completedCount / total) * 100) : 0;

  // Active selected subtask from detail
  const activeSelectedSubtask = useMemo(() => {
    if (!selectedSubtask || !taskDetail) return null;
    return (
      (taskDetail.subtasks ?? []).find((s) => s.id === selectedSubtask.id) ??
      selectedSubtask
    );
  }, [selectedSubtask, taskDetail]);

  if (tasks.length === 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center px-4 text-center">
        <Clock className="mb-2 size-8 text-gray-600" />
        <p className="text-sm text-muted-foreground">Waiting for task...</p>
        <p className="mt-1 text-xs text-gray-600">
          Send a message to start a new task
        </p>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-border px-3 py-2">
        <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          Progress
        </span>
        <div className="flex items-center gap-1.5">
          {/* Task dropdown */}
          <div className="relative">
            <button
              onClick={() => setShowDropdown(!showDropdown)}
              className="flex items-center gap-1 rounded px-2 py-1 text-xs text-muted-foreground transition-colors hover:text-foreground hover:bg-secondary/50"
            >
              <span className="max-w-[120px] truncate">
                {selectedTask?.title ?? "Select task"}
              </span>
              <ChevronDown className="size-3" />
            </button>
            {showDropdown && (
              <div className="absolute right-0 top-full mt-1 z-20 w-64 rounded-lg border border-border bg-card p-1 shadow-lg">
                {tasks.map((t) => {
                  const cfg = taskStatusConfig[t.status];
                  return (
                    <button
                      key={t.id}
                      onClick={() => {
                        setSelectedTaskId(t.id);
                        setShowDropdown(false);
                      }}
                      className={cn(
                        "flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-xs transition-colors",
                        t.id === selectedTaskId
                          ? "bg-secondary text-foreground"
                          : "text-muted-foreground hover:bg-secondary/50",
                      )}
                    >
                      <span className={cn("shrink-0", cfg.className)}>
                        {cfg.label}
                      </span>
                      <span className="flex-1 truncate text-foreground">
                        {t.title}
                      </span>
                    </button>
                  );
                })}
              </div>
            )}
          </div>
          <a
            href={selectedTaskId ? `/tasks/${selectedTaskId}` : "#"}
            target="_blank"
            rel="noopener noreferrer"
            className="rounded p-1 text-muted-foreground transition-colors hover:text-foreground"
            title="Open task detail"
          >
            <ExternalLink className="size-3.5" />
          </a>
        </div>
      </div>

      {/* Subtask waves / Plan preview */}
      <div className="flex-1 overflow-y-auto px-3 py-2">
        {isLoadingDetail && subtasks.length === 0 && !planPreview ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="size-5 animate-spin text-muted-foreground" />
          </div>
        ) : planPreview && planWaves.length > 0 ? (
          <>
            {planPreview.summary && (
              <p className="mb-2 text-xs text-muted-foreground">
                {planPreview.summary}
              </p>
            )}
            {planWaves.map(({ wave, items }) => (
              <div key={wave} className="mb-3">
                <div className="mb-1 text-[10px] font-semibold uppercase tracking-wider text-gray-500">
                  Wave {wave + 1}
                </div>
                <div className="space-y-1">
                  {items.map((st) => {
                    const planSt = planPreview.subtasks.find((p) => p.id === st.id);
                    const agentName = planSt?.agent_name || st.agent_id.slice(0, 8);
                    return (
                      <div
                        key={st.id}
                        className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs"
                      >
                        <Clock className="size-3.5 shrink-0 text-gray-400" />
                        <span className="flex-1 truncate text-foreground">
                          {agentName}
                        </span>
                        <span className="truncate text-muted-foreground max-w-[100px]">
                          {st.instruction.length > 30
                            ? `${st.instruction.slice(0, 30)}...`
                            : st.instruction}
                        </span>
                      </div>
                    );
                  })}
                </div>
              </div>
            ))}
          </>
        ) : (
          waves.map(({ wave, items }) => (
            <div key={wave} className="mb-3">
              <div className="mb-1 text-[10px] font-semibold uppercase tracking-wider text-gray-500">
                Wave {wave + 1}
              </div>
              <div className="space-y-1">
                {items.map((st) => {
                  const Icon = subtaskIcons[st.status] ?? Clock;
                  const color = subtaskColors[st.status] ?? "text-gray-400";
                  const agentName =
                    allAgents.find((a) => a.id === st.agent_id)?.name ??
                    st.agent_id.slice(0, 8);
                  return (
                    <button
                      key={st.id}
                      onClick={() => setSelectedSubtask(st)}
                      className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs transition-colors hover:bg-secondary/50"
                    >
                      <Icon
                        className={cn(
                          "size-3.5 shrink-0",
                          color,
                          st.status === "running" && "animate-spin",
                        )}
                      />
                      <span className="flex-1 truncate text-foreground">
                        {agentName}
                      </span>
                      <span className="truncate text-muted-foreground max-w-[100px]">
                        {st.instruction.length > 30
                          ? `${st.instruction.slice(0, 30)}...`
                          : st.instruction}
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>
          ))
        )}
      </div>

      {/* Progress bar */}
      {total > 0 && (
        <div className="border-t border-border px-3 py-2">
          <div className="flex items-center justify-between text-xs text-muted-foreground mb-1">
            <span>
              {completedCount}/{total}
            </span>
            <span>{pct}%</span>
          </div>
          <div className="h-1.5 w-full rounded-full bg-gray-800">
            <div
              className="h-1.5 rounded-full bg-green-500 transition-all"
              style={{ width: `${pct}%` }}
            />
          </div>
        </div>
      )}

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
    </div>
  );
}
