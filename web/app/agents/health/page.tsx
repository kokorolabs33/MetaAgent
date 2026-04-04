"use client";

import { useEffect, useState, useMemo } from "react";
import {
  HeartPulse,
  Loader2,
  AlertCircle,
  ArrowUpDown,
  Clock,
} from "lucide-react";
import { api } from "@/lib/api";
import type { AgentHealthOverview } from "@/lib/types";
import { cn } from "@/lib/utils";

type SortKey =
  | "name"
  | "status"
  | "total_subtasks"
  | "success_rate"
  | "avg_duration_sec";
type SortDir = "asc" | "desc";

function formatDuration(seconds: number): string {
  if (seconds === 0) return "-";
  if (seconds < 60) return `${Math.round(seconds)}s`;
  const min = Math.floor(seconds / 60);
  const sec = Math.round(seconds % 60);
  if (min < 60) return `${min}m ${sec}s`;
  const hr = Math.floor(min / 60);
  const remMin = min % 60;
  return `${hr}h ${remMin}m`;
}

function formatTime(ts: string | null): string {
  if (!ts) return "Never";
  const d = new Date(ts);
  const now = new Date();
  const diffMs = now.getTime() - d.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return "Just now";
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  return d.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default function AgentHealthPage() {
  const [agents, setAgents] = useState<AgentHealthOverview[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [sortKey, setSortKey] = useState<SortKey>("total_subtasks");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  useEffect(() => {
    const load = async () => {
      try {
        const result = await api.agents.healthOverview();
        setAgents(result);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Failed to load agent health",
        );
      } finally {
        setIsLoading(false);
      }
    };
    void load();
  }, []);

  const sorted = useMemo(() => {
    return [...agents].sort((a, b) => {
      let cmp = 0;
      switch (sortKey) {
        case "name":
          cmp = a.name.localeCompare(b.name);
          break;
        case "status":
          cmp = Number(b.is_online) - Number(a.is_online);
          break;
        case "total_subtasks":
          cmp = a.total_subtasks - b.total_subtasks;
          break;
        case "success_rate":
          cmp = a.success_rate - b.success_rate;
          break;
        case "avg_duration_sec":
          cmp = a.avg_duration_sec - b.avg_duration_sec;
          break;
      }
      return sortDir === "asc" ? cmp : -cmp;
    });
  }, [agents, sortKey, sortDir]);

  function toggleSort(key: SortKey) {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("desc");
    }
  }

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="flex items-center gap-2 text-red-400">
          <AlertCircle className="size-5" />
          <span>{error}</span>
        </div>
      </div>
    );
  }

  const onlineCount = agents.filter((a) => a.is_online).length;
  const totalTasks = agents.reduce((s, a) => s + a.total_subtasks, 0);
  const totalCompleted = agents.reduce((s, a) => s + a.completed, 0);
  const overallRate = totalTasks > 0 ? totalCompleted / totalTasks : 0;

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center gap-3">
        <HeartPulse className="size-6 text-rose-400" />
        <h1 className="text-2xl font-bold text-foreground">Agent Health</h1>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="rounded-lg border border-border bg-gray-900/50 p-5">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-muted-foreground">
              Agents Online
            </span>
            <span
              className={cn(
                "size-3 rounded-full",
                onlineCount > 0 ? "bg-green-500" : "bg-red-500",
              )}
            />
          </div>
          <div className="mt-2 text-3xl font-bold text-foreground">
            {onlineCount}
            <span className="text-lg font-normal text-muted-foreground">
              /{agents.length}
            </span>
          </div>
        </div>

        <div className="rounded-lg border border-border bg-gray-900/50 p-5">
          <span className="text-sm font-medium text-muted-foreground">
            Total Subtasks
          </span>
          <div className="mt-2 text-3xl font-bold text-foreground">
            {totalTasks}
          </div>
        </div>

        <div className="rounded-lg border border-border bg-gray-900/50 p-5">
          <span className="text-sm font-medium text-muted-foreground">
            Overall Success Rate
          </span>
          <div
            className={cn(
              "mt-2 text-3xl font-bold",
              overallRate > 0.8
                ? "text-green-400"
                : overallRate > 0.5
                  ? "text-amber-400"
                  : "text-red-400",
            )}
          >
            {(overallRate * 100).toFixed(1)}%
          </div>
        </div>

        <div className="rounded-lg border border-border bg-gray-900/50 p-5">
          <div className="flex items-center gap-2">
            <Clock className="size-4 text-cyan-400" />
            <span className="text-sm font-medium text-muted-foreground">
              Avg Response
            </span>
          </div>
          <div className="mt-2 text-3xl font-bold text-foreground">
            {agents.length > 0
              ? formatDuration(
                  agents.reduce((s, a) => s + a.avg_duration_sec, 0) /
                    agents.filter((a) => a.avg_duration_sec > 0).length || 0,
                )
              : "-"}
          </div>
        </div>
      </div>

      {/* Agent table */}
      <div className="rounded-lg border border-border bg-gray-900/50">
        {agents.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs text-muted-foreground">
                  <th className="px-4 py-3 font-medium">Status</th>
                  <SortHeader
                    label="Agent"
                    sortKey="name"
                    currentKey={sortKey}
                    dir={sortDir}
                    onSort={toggleSort}
                  />
                  <th className="px-4 py-3 font-medium">Last Check</th>
                  <SortHeader
                    label="Total Tasks"
                    sortKey="total_subtasks"
                    currentKey={sortKey}
                    dir={sortDir}
                    onSort={toggleSort}
                  />
                  <th className="px-4 py-3 font-medium">Completed</th>
                  <th className="px-4 py-3 font-medium">Failed</th>
                  <SortHeader
                    label="Success Rate"
                    sortKey="success_rate"
                    currentKey={sortKey}
                    dir={sortDir}
                    onSort={toggleSort}
                  />
                  <SortHeader
                    label="Avg Duration"
                    sortKey="avg_duration_sec"
                    currentKey={sortKey}
                    dir={sortDir}
                    onSort={toggleSort}
                  />
                </tr>
              </thead>
              <tbody>
                {sorted.map((agent) => (
                  <tr
                    key={agent.id}
                    className="border-b border-border/50 hover:bg-gray-800/30"
                  >
                    <td className="px-4 py-3">
                      <span
                        className={cn(
                          "inline-block size-2.5 rounded-full",
                          agent.is_online ? "bg-green-500" : "bg-red-500",
                        )}
                        title={agent.is_online ? "Online" : "Offline"}
                      />
                    </td>
                    <td className="px-4 py-3 font-medium text-foreground">
                      {agent.name}
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {formatTime(agent.last_health_check)}
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {agent.total_subtasks}
                    </td>
                    <td className="px-4 py-3 text-green-400">
                      {agent.completed}
                    </td>
                    <td className="px-4 py-3 text-red-400">{agent.failed}</td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <div className="h-2 w-20 rounded-full bg-gray-800">
                          <div
                            className={cn(
                              "h-2 rounded-full transition-all",
                              agent.success_rate > 0.8
                                ? "bg-green-500"
                                : agent.success_rate > 0.5
                                  ? "bg-amber-500"
                                  : "bg-red-500",
                            )}
                            style={{
                              width: `${agent.success_rate * 100}%`,
                            }}
                          />
                        </div>
                        <span
                          className={cn(
                            "text-xs font-medium",
                            agent.success_rate > 0.8
                              ? "text-green-400"
                              : agent.success_rate > 0.5
                                ? "text-amber-400"
                                : "text-red-400",
                          )}
                        >
                          {(agent.success_rate * 100).toFixed(0)}%
                        </span>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {formatDuration(agent.avg_duration_sec)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="flex h-40 items-center justify-center">
            <p className="text-sm text-muted-foreground">
              No active agents found
            </p>
          </div>
        )}
      </div>
    </div>
  );
}

function SortHeader({
  label,
  sortKey,
  currentKey,
  dir,
  onSort,
}: {
  label: string;
  sortKey: SortKey;
  currentKey: SortKey;
  dir: SortDir;
  onSort: (key: SortKey) => void;
}) {
  return (
    <th className="px-4 py-3 font-medium">
      <button
        onClick={() => onSort(sortKey)}
        className="flex items-center gap-1 hover:text-foreground transition-colors"
      >
        {label}
        <ArrowUpDown
          className={cn(
            "size-3",
            currentKey === sortKey
              ? "text-foreground"
              : "text-muted-foreground/50",
          )}
        />
        {currentKey === sortKey && (
          <span className="text-[10px]">{dir === "asc" ? "^" : "v"}</span>
        )}
      </button>
    </th>
  );
}
