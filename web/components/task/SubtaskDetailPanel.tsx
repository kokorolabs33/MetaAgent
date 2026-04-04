"use client";

import { useEffect, useRef } from "react";
import {
  X,
  Clock,
  CheckCircle2,
  XCircle,
  AlertCircle,
  RotateCw,
  Play,
  Pause,
  Ban,
  CircleDot,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { SubTask } from "@/lib/types";

interface SubtaskDetailPanelProps {
  subtask: SubTask;
  agentName: string;
  onClose: () => void;
}

const statusConfig: Record<
  SubTask["status"],
  { label: string; className: string; icon: typeof CheckCircle2 }
> = {
  pending: { label: "Pending", className: "bg-gray-500/20 text-gray-400", icon: Clock },
  running: { label: "Running", className: "bg-blue-500/20 text-blue-400", icon: Play },
  completed: { label: "Completed", className: "bg-green-500/20 text-green-400", icon: CheckCircle2 },
  failed: { label: "Failed", className: "bg-red-500/20 text-red-400", icon: XCircle },
  input_required: { label: "Input Required", className: "bg-purple-500/20 text-purple-400", icon: AlertCircle },
  approval_required: { label: "Approval Required", className: "bg-amber-500/20 text-amber-400", icon: Pause },
  cancelled: { label: "Cancelled", className: "bg-gray-500/20 text-gray-500", icon: Ban },
  blocked: { label: "Blocked", className: "bg-orange-500/20 text-orange-400", icon: CircleDot },
};

function formatTimestamp(iso: string): string {
  return new Date(iso).toLocaleString();
}

function formatDurationBetween(start: string, end: string): string {
  const diffMs = new Date(end).getTime() - new Date(start).getTime();
  if (diffMs < 0) return "0s";
  const totalSec = Math.floor(diffMs / 1000);
  if (totalSec < 60) return `${totalSec}s`;
  const min = Math.floor(totalSec / 60);
  const sec = totalSec % 60;
  if (min < 60) return `${min}m ${sec}s`;
  const hr = Math.floor(min / 60);
  const remMin = min % 60;
  return `${hr}h ${remMin}m`;
}

function JsonBlock({ data }: { data: Record<string, unknown> }) {
  return (
    <pre className="max-h-48 overflow-auto rounded-md bg-gray-900/80 p-3 text-xs text-gray-300">
      {JSON.stringify(data, null, 2)}
    </pre>
  );
}

export function SubtaskDetailPanel({
  subtask,
  agentName,
  onClose,
}: SubtaskDetailPanelProps) {
  const panelRef = useRef<HTMLDivElement>(null);

  // Close on click outside
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (panelRef.current && !panelRef.current.contains(e.target as Node)) {
        onClose();
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [onClose]);

  // Close on Escape
  useEffect(() => {
    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [onClose]);

  const config = statusConfig[subtask.status];
  const StatusIcon = config.icon;

  return (
    <div className="fixed inset-0 z-40">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/40" />

      {/* Panel */}
      <div
        ref={panelRef}
        className="absolute right-0 top-0 h-full w-full max-w-md overflow-y-auto border-l border-border bg-gray-950 shadow-2xl animate-in slide-in-from-right"
      >
        {/* Header */}
        <div className="sticky top-0 z-10 flex items-center justify-between border-b border-border bg-gray-950 px-4 py-3">
          <div className="flex items-center gap-2 overflow-hidden">
            <span className="truncate text-sm font-semibold text-foreground">
              {agentName}
            </span>
            <Badge className={config.className}>
              <StatusIcon className="mr-1 size-3" />
              {config.label}
            </Badge>
          </div>
          <button
            onClick={onClose}
            className="rounded-md p-1 text-muted-foreground transition-colors hover:text-foreground"
          >
            <X className="size-5" />
          </button>
        </div>

        <div className="space-y-5 p-4">
          {/* Instruction */}
          <section>
            <h3 className="mb-1.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Instruction
            </h3>
            <p className="whitespace-pre-wrap text-sm text-foreground">
              {subtask.instruction}
            </p>
          </section>

          {/* Dependencies */}
          {(subtask.depends_on ?? []).length > 0 && (
            <section>
              <h3 className="mb-1.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Dependencies
              </h3>
              <div className="flex flex-wrap gap-1.5">
                {(subtask.depends_on ?? []).map((depId) => (
                  <Badge
                    key={depId}
                    className="bg-gray-800 text-gray-300 font-mono text-[10px]"
                  >
                    {depId.slice(0, 8)}...
                  </Badge>
                ))}
              </div>
            </section>
          )}

          {/* Attempt info */}
          <section>
            <h3 className="mb-1.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Attempts
            </h3>
            <div className="flex items-center gap-2 text-sm text-foreground">
              <RotateCw className="size-3.5 text-muted-foreground" />
              <span>
                {subtask.attempt} / {subtask.max_attempts}
              </span>
            </div>
          </section>

          {/* Matched skills */}
          {subtask.matched_skills && subtask.matched_skills.length > 0 && (
            <section>
              <h3 className="mb-1.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Matched Skills
              </h3>
              <div className="flex flex-wrap gap-1.5">
                {subtask.matched_skills.map((skill) => (
                  <Badge
                    key={skill}
                    className="bg-indigo-500/20 text-indigo-400"
                  >
                    {skill}
                  </Badge>
                ))}
              </div>
            </section>
          )}

          {/* Input */}
          {subtask.input && Object.keys(subtask.input).length > 0 && (
            <section>
              <h3 className="mb-1.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Input
              </h3>
              <JsonBlock data={subtask.input} />
            </section>
          )}

          {/* Output */}
          {subtask.output && Object.keys(subtask.output).length > 0 && (
            <section>
              <h3 className="mb-1.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Output
              </h3>
              <JsonBlock data={subtask.output} />
            </section>
          )}

          {/* Error */}
          {subtask.error && (
            <section>
              <h3 className="mb-1.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Error
              </h3>
              <div className="rounded-md border border-red-500/30 bg-red-950/30 p-3 text-sm text-red-400">
                {subtask.error}
              </div>
            </section>
          )}

          {/* A2A Task ID */}
          {subtask.a2a_task_id && (
            <section>
              <h3 className="mb-1.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                A2A Task ID
              </h3>
              <code className="text-xs text-muted-foreground">
                {subtask.a2a_task_id}
              </code>
            </section>
          )}

          {/* Timing */}
          <section>
            <h3 className="mb-1.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Timing
            </h3>
            <div className="space-y-1.5 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Created</span>
                <span className="text-foreground">
                  {formatTimestamp(subtask.created_at)}
                </span>
              </div>
              {subtask.started_at && (
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Started</span>
                  <span className="text-foreground">
                    {formatTimestamp(subtask.started_at)}
                  </span>
                </div>
              )}
              {subtask.completed_at && (
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Completed</span>
                  <span className="text-foreground">
                    {formatTimestamp(subtask.completed_at)}
                  </span>
                </div>
              )}
              {subtask.started_at && subtask.completed_at && (
                <div className="flex justify-between border-t border-border pt-1.5">
                  <span className="text-muted-foreground">Duration</span>
                  <span className="font-medium text-foreground">
                    {formatDurationBetween(
                      subtask.started_at,
                      subtask.completed_at,
                    )}
                  </span>
                </div>
              )}
              {subtask.started_at && !subtask.completed_at && (
                <div className="flex justify-between border-t border-border pt-1.5">
                  <span className="text-muted-foreground">Elapsed</span>
                  <span className="font-medium text-foreground">
                    {formatDurationBetween(
                      subtask.started_at,
                      new Date().toISOString(),
                    )}
                  </span>
                </div>
              )}
            </div>
          </section>

          {/* Attempt history */}
          {subtask.attempt_history && subtask.attempt_history.length > 0 && (
            <section>
              <h3 className="mb-1.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Attempt History
              </h3>
              <div className="space-y-2">
                {subtask.attempt_history.map((attempt, idx) => (
                  <div
                    key={idx}
                    className="rounded-md border border-border bg-gray-900/50 p-2"
                  >
                    <div className="mb-1 text-xs font-medium text-muted-foreground">
                      Attempt {idx + 1}
                    </div>
                    <pre className="max-h-24 overflow-auto text-xs text-gray-300">
                      {JSON.stringify(attempt, null, 2)}
                    </pre>
                  </div>
                ))}
              </div>
            </section>
          )}
        </div>
      </div>
    </div>
  );
}
