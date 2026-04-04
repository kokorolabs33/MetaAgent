"use client";

import { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import {
  ScrollText,
  Loader2,
  AlertCircle,
  ChevronLeft,
  ChevronRight,
  ChevronDown,
  ChevronUp,
  Search,
} from "lucide-react";
import { api } from "@/lib/api";
import type { AuditLogEntry, PaginatedAuditLogs } from "@/lib/types";
import { cn } from "@/lib/utils";

const EVENT_TYPE_COLORS: Record<string, string> = {
  task_started: "bg-blue-500/20 text-blue-400",
  task_completed: "bg-green-500/20 text-green-400",
  task_failed: "bg-red-500/20 text-red-400",
  task_cancelled: "bg-gray-500/20 text-gray-400",
  subtask_started: "bg-cyan-500/20 text-cyan-400",
  subtask_completed: "bg-emerald-500/20 text-emerald-400",
  subtask_failed: "bg-red-500/20 text-red-400",
  agent_joined: "bg-purple-500/20 text-purple-400",
  agent_working: "bg-amber-500/20 text-amber-400",
  agent_done: "bg-green-500/20 text-green-400",
  message: "bg-indigo-500/20 text-indigo-400",
  plan_created: "bg-violet-500/20 text-violet-400",
  approval_required: "bg-amber-500/20 text-amber-400",
};

function formatTimestamp(ts: string): string {
  const d = new Date(ts);
  return d.toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function getTypeColor(type: string): string {
  return EVENT_TYPE_COLORS[type] ?? "bg-gray-500/20 text-gray-400";
}

export default function AuditLogPage() {
  const [data, setData] = useState<PaginatedAuditLogs | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set());

  // Filters
  const [taskFilter, setTaskFilter] = useState("");
  const [agentFilter, setAgentFilter] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [page, setPage] = useState(1);
  const perPage = 50;

  const loadData = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await api.auditLogs.list({
        task_id: taskFilter || undefined,
        agent_id: agentFilter || undefined,
        type: typeFilter || undefined,
        page,
        per_page: perPage,
      });
      setData(result);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to load audit logs",
      );
    } finally {
      setIsLoading(false);
    }
  }, [taskFilter, agentFilter, typeFilter, page]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  function toggleRow(id: string) {
    setExpandedRows((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  function handleFilterSubmit(e: React.FormEvent) {
    e.preventDefault();
    setPage(1);
    void loadData();
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center gap-3">
        <ScrollText className="size-6 text-indigo-400" />
        <h1 className="text-2xl font-bold text-foreground">Audit Log</h1>
      </div>

      {/* Filters */}
      <form
        onSubmit={handleFilterSubmit}
        className="flex flex-wrap items-end gap-3"
      >
        <div>
          <label className="mb-1 block text-xs font-medium text-muted-foreground">
            Event Type
          </label>
          <input
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            placeholder="e.g. task_completed"
            className="w-48 rounded-lg border border-input bg-transparent px-3 py-1.5 text-sm text-foreground placeholder:text-muted-foreground transition-colors focus-visible:border-ring focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-muted-foreground">
            Task ID
          </label>
          <input
            value={taskFilter}
            onChange={(e) => setTaskFilter(e.target.value)}
            placeholder="Filter by task ID"
            className="w-48 rounded-lg border border-input bg-transparent px-3 py-1.5 text-sm text-foreground placeholder:text-muted-foreground transition-colors focus-visible:border-ring focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-muted-foreground">
            Agent / Actor ID
          </label>
          <input
            value={agentFilter}
            onChange={(e) => setAgentFilter(e.target.value)}
            placeholder="Filter by actor ID"
            className="w-48 rounded-lg border border-input bg-transparent px-3 py-1.5 text-sm text-foreground placeholder:text-muted-foreground transition-colors focus-visible:border-ring focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
          />
        </div>
        <button
          type="submit"
          className="flex items-center gap-1.5 rounded-lg bg-secondary px-4 py-1.5 text-sm font-medium text-white hover:bg-secondary/80 transition-colors"
        >
          <Search className="size-3.5" />
          Filter
        </button>
      </form>

      {/* Error state */}
      {error && (
        <div className="flex items-center gap-2 text-red-400">
          <AlertCircle className="size-5" />
          <span>{error}</span>
        </div>
      )}

      {/* Table */}
      <div className="rounded-lg border border-border bg-gray-900/50">
        {isLoading ? (
          <div className="flex h-40 items-center justify-center">
            <Loader2 className="size-6 animate-spin text-muted-foreground" />
          </div>
        ) : data && data.items.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs text-muted-foreground">
                  <th className="w-8 px-4 py-3" />
                  <th className="px-4 py-3 font-medium">Timestamp</th>
                  <th className="px-4 py-3 font-medium">Event Type</th>
                  <th className="px-4 py-3 font-medium">Actor</th>
                  <th className="px-4 py-3 font-medium">Task</th>
                  <th className="px-4 py-3 font-medium">Subtask</th>
                </tr>
              </thead>
              <tbody>
                {data.items.map((entry) => (
                  <AuditRow
                    key={entry.id}
                    entry={entry}
                    isExpanded={expandedRows.has(entry.id)}
                    onToggle={() => toggleRow(entry.id)}
                  />
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="flex h-40 items-center justify-center">
            <p className="text-sm text-muted-foreground">
              {data ? "No events match the current filters" : "No data"}
            </p>
          </div>
        )}
      </div>

      {/* Pagination */}
      {data && data.pages > 1 && (
        <div className="flex items-center justify-between">
          <span className="text-xs text-muted-foreground">
            Page {data.page} of {data.pages} ({data.total} total events)
          </span>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page <= 1}
              className="flex items-center gap-1 rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-muted-foreground hover:bg-secondary/50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              <ChevronLeft className="size-3.5" />
              Previous
            </button>
            <button
              onClick={() => setPage((p) => Math.min(data.pages, p + 1))}
              disabled={page >= data.pages}
              className="flex items-center gap-1 rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-muted-foreground hover:bg-secondary/50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              Next
              <ChevronRight className="size-3.5" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function AuditRow({
  entry,
  isExpanded,
  onToggle,
}: {
  entry: AuditLogEntry;
  isExpanded: boolean;
  onToggle: () => void;
}) {
  const hasData =
    entry.data && Object.keys(entry.data).length > 0;

  return (
    <>
      <tr className="border-b border-border/50 hover:bg-gray-800/30">
        <td className="px-4 py-3">
          {hasData && (
            <button
              onClick={onToggle}
              className="text-muted-foreground hover:text-foreground transition-colors"
            >
              {isExpanded ? (
                <ChevronUp className="size-4" />
              ) : (
                <ChevronDown className="size-4" />
              )}
            </button>
          )}
        </td>
        <td className="px-4 py-3 text-muted-foreground whitespace-nowrap">
          {formatTimestamp(entry.created_at)}
        </td>
        <td className="px-4 py-3">
          <span
            className={cn(
              "rounded-full px-2.5 py-0.5 text-xs font-medium",
              getTypeColor(entry.type),
            )}
          >
            {entry.type}
          </span>
        </td>
        <td className="px-4 py-3 text-muted-foreground">
          <span className="text-xs text-muted-foreground/60">
            {entry.actor_type}
          </span>
          {entry.actor_id && (
            <span className="ml-1 font-mono text-xs text-foreground/80">
              {entry.actor_id.length > 12
                ? `${entry.actor_id.slice(0, 12)}...`
                : entry.actor_id}
            </span>
          )}
        </td>
        <td className="px-4 py-3">
          <Link
            href={`/tasks/${entry.task_id}`}
            className="font-mono text-xs text-blue-400 hover:underline"
          >
            {entry.task_id.slice(0, 8)}...
          </Link>
        </td>
        <td className="px-4 py-3 text-muted-foreground font-mono text-xs">
          {entry.subtask_id
            ? `${entry.subtask_id.slice(0, 8)}...`
            : "-"}
        </td>
      </tr>
      {isExpanded && hasData && (
        <tr className="border-b border-border/50 bg-gray-800/20">
          <td colSpan={6} className="px-8 py-3">
            <pre className="max-h-60 overflow-auto rounded-lg bg-gray-900 p-3 text-xs text-gray-300 font-mono">
              {JSON.stringify(entry.data, null, 2)}
            </pre>
          </td>
        </tr>
      )}
    </>
  );
}
