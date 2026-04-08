"use client";

import { useMemo } from "react";
import { Clock, CheckCircle2, XCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { Task, ExecutionPlan, PlanSubTask } from "@/lib/types";

interface PlanReviewCardProps {
  task: Task;
  isApproving: boolean;
  onApprove: () => void;
  onReject: () => void;
}

function groupPlanIntoWaves(subtasks: PlanSubTask[]) {
  const waveMap = new Map<number, PlanSubTask[]>();
  const cache = new Map<string, number>();

  function getWave(st: PlanSubTask): number {
    if (cache.has(st.id)) return cache.get(st.id)!;
    if (!st.depends_on?.length) {
      cache.set(st.id, 0);
      return 0;
    }
    const depWaves = st.depends_on.map((depId) => {
      const dep = subtasks.find((s) => s.id === depId);
      return dep ? getWave(dep) : 0;
    });
    const w = Math.max(...depWaves) + 1;
    cache.set(st.id, w);
    return w;
  }

  for (const st of subtasks) {
    const w = getWave(st);
    if (!waveMap.has(w)) waveMap.set(w, []);
    waveMap.get(w)!.push(st);
  }

  return Array.from(waveMap.entries())
    .sort(([a], [b]) => a - b)
    .map(([wave, items]) => ({ wave, items }));
}

export function PlanReviewCard({
  task,
  isApproving,
  onApprove,
  onReject,
}: PlanReviewCardProps) {
  const plan = useMemo((): ExecutionPlan | null => {
    const raw = task.plan as unknown as ExecutionPlan | undefined;
    if (!raw || !Array.isArray(raw.subtasks)) return null;
    return raw;
  }, [task.plan]);

  const waves = useMemo(() => {
    if (!plan) return [];
    return groupPlanIntoWaves(plan.subtasks);
  }, [plan]);

  if (!plan || waves.length === 0) return null;

  return (
    <div className="mx-4 my-3 rounded-xl border border-amber-500/30 bg-amber-500/5">
      {/* Header */}
      <div className="flex items-center gap-2 border-b border-amber-500/20 px-4 py-3">
        <Clock className="size-4 text-amber-400" />
        <span className="text-sm font-semibold text-amber-300">
          Execution Plan
        </span>
        <span className="text-xs text-muted-foreground">
          {plan.subtasks.length} subtasks across {waves.length} waves
        </span>
      </div>

      {/* Summary */}
      {plan.summary && (
        <p className="px-4 pt-3 text-sm text-muted-foreground">
          {plan.summary}
        </p>
      )}

      {/* Waves */}
      <div className="px-4 py-3 space-y-4">
        {waves.map(({ wave, items }) => (
          <div key={wave}>
            <div className="mb-2 text-xs font-semibold uppercase tracking-wider text-gray-500">
              Wave {wave + 1}
              {items.length > 1 && (
                <span className="ml-1 font-normal normal-case text-gray-600">
                  (parallel)
                </span>
              )}
            </div>
            <div className="space-y-2">
              {items.map((st) => (
                <div
                  key={st.id}
                  className="flex items-start gap-3 rounded-lg bg-gray-800/50 px-3 py-2.5"
                >
                  <div className="mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full bg-gray-700 text-[10px] font-bold text-gray-300">
                    {st.agent_name?.[0]?.toUpperCase() ?? "?"}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="text-xs font-medium text-foreground">
                      {st.agent_name || st.agent_id}
                    </div>
                    <div className="mt-0.5 text-xs text-muted-foreground leading-relaxed">
                      {st.instruction}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>

      {/* Actions */}
      <div className="flex items-center justify-end gap-2 border-t border-amber-500/20 px-4 py-3">
        <Button
          variant="ghost"
          size="sm"
          onClick={onReject}
          disabled={isApproving}
          className="text-muted-foreground hover:text-foreground"
        >
          <XCircle className="mr-1.5 size-3.5" />
          Reject
        </Button>
        <Button
          size="sm"
          onClick={onApprove}
          disabled={isApproving}
        >
          <CheckCircle2 className="mr-1.5 size-3.5" />
          {isApproving ? "Approving..." : "Approve & Execute"}
        </Button>
      </div>
    </div>
  );
}
