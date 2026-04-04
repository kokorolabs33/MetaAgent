"use client";

import { useEffect, useState } from "react";
import {
  BarChart3,
  CheckCircle2,
  XCircle,
  Users,
  Clock,
  Loader2,
  AlertCircle,
} from "lucide-react";
import { api } from "@/lib/api";
import type { DashboardData } from "@/lib/types";
import { cn } from "@/lib/utils";

const statusColors: Record<string, string> = {
  completed: "bg-green-500",
  running: "bg-amber-500",
  planning: "bg-blue-500",
  pending: "bg-gray-500",
  failed: "bg-red-500",
  cancelled: "bg-gray-600",
  approval_required: "bg-amber-600",
};

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`;
  const min = Math.floor(seconds / 60);
  const sec = Math.round(seconds % 60);
  if (min < 60) return `${min}m ${sec}s`;
  const hr = Math.floor(min / 60);
  const remMin = min % 60;
  return `${hr}h ${remMin}m`;
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr);
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

export default function AnalyticsPage() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const load = async () => {
      try {
        const result = await api.analytics.dashboard();
        setData(result);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load analytics");
      } finally {
        setIsLoading(false);
      }
    };
    void load();
  }, []);

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="flex items-center gap-2 text-red-400">
          <AlertCircle className="size-5" />
          <span>{error ?? "No data available"}</span>
        </div>
      </div>
    );
  }

  const maxDaily = Math.max(...(data.daily_tasks ?? []).map((d) => d.count), 1);
  const maxStatusCount = Math.max(
    ...(data.status_distribution ?? []).map((s) => s.count),
    1,
  );

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center gap-3">
        <BarChart3 className="size-6 text-blue-400" />
        <h1 className="text-2xl font-bold text-foreground">Analytics</h1>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {/* Total Tasks */}
        <div className="rounded-lg border border-border bg-gray-900/50 p-5">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-muted-foreground">
              Total Tasks
            </span>
            <BarChart3 className="size-4 text-blue-400" />
          </div>
          <div className="mt-2 text-3xl font-bold text-foreground">
            {data.total_tasks}
          </div>
          <div className="mt-1 flex gap-3 text-xs text-muted-foreground">
            <span className="flex items-center gap-1">
              <CheckCircle2 className="size-3 text-green-400" />
              {data.completed_tasks} completed
            </span>
            <span className="flex items-center gap-1">
              <XCircle className="size-3 text-red-400" />
              {data.failed_tasks} failed
            </span>
          </div>
        </div>

        {/* Success Rate */}
        <div className="rounded-lg border border-border bg-gray-900/50 p-5">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-muted-foreground">
              Success Rate
            </span>
            <CheckCircle2
              className={cn(
                "size-4",
                data.success_rate > 0.8
                  ? "text-green-400"
                  : data.success_rate > 0.5
                    ? "text-amber-400"
                    : "text-red-400",
              )}
            />
          </div>
          <div
            className={cn(
              "mt-2 text-3xl font-bold",
              data.success_rate > 0.8
                ? "text-green-400"
                : data.success_rate > 0.5
                  ? "text-amber-400"
                  : "text-red-400",
            )}
          >
            {(data.success_rate * 100).toFixed(1)}%
          </div>
          <div className="mt-1 text-xs text-muted-foreground">
            {data.running_tasks} currently running
          </div>
        </div>

        {/* Active Agents */}
        <div className="rounded-lg border border-border bg-gray-900/50 p-5">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-muted-foreground">
              Active Agents
            </span>
            <Users className="size-4 text-purple-400" />
          </div>
          <div className="mt-2 text-3xl font-bold text-foreground">
            {data.online_agents}
            <span className="text-lg font-normal text-muted-foreground">
              /{data.total_agents}
            </span>
          </div>
          <div className="mt-1 text-xs text-muted-foreground">
            online / total registered
          </div>
        </div>

        {/* Avg Duration */}
        <div className="rounded-lg border border-border bg-gray-900/50 p-5">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-muted-foreground">
              Avg Duration
            </span>
            <Clock className="size-4 text-cyan-400" />
          </div>
          <div className="mt-2 text-3xl font-bold text-foreground">
            {formatDuration(data.avg_duration_sec)}
          </div>
          <div className="mt-1 text-xs text-muted-foreground">
            per completed task
          </div>
        </div>
      </div>

      {/* Charts row */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Status distribution */}
        <div className="rounded-lg border border-border bg-gray-900/50 p-5">
          <h2 className="mb-4 text-sm font-semibold text-foreground">
            Status Distribution
          </h2>
          <div className="space-y-3">
            {(data.status_distribution ?? []).map((item) => (
              <div key={item.status} className="space-y-1">
                <div className="flex items-center justify-between text-xs">
                  <span className="capitalize text-muted-foreground">
                    {item.status.replace("_", " ")}
                  </span>
                  <span className="font-medium text-foreground">
                    {item.count}
                  </span>
                </div>
                <div className="h-2 w-full rounded-full bg-gray-800">
                  <div
                    className={cn(
                      "h-2 rounded-full transition-all",
                      statusColors[item.status] ?? "bg-gray-500",
                    )}
                    style={{
                      width: `${(item.count / maxStatusCount) * 100}%`,
                    }}
                  />
                </div>
              </div>
            ))}
            {(data.status_distribution ?? []).length === 0 && (
              <p className="text-xs text-muted-foreground">No tasks yet</p>
            )}
          </div>
        </div>

        {/* Daily tasks bar chart */}
        <div className="rounded-lg border border-border bg-gray-900/50 p-5">
          <h2 className="mb-4 text-sm font-semibold text-foreground">
            Tasks (Last 7 Days)
          </h2>
          {(data.daily_tasks ?? []).length > 0 ? (
            <div className="flex h-40 items-end gap-2">
              {(data.daily_tasks ?? []).map((day) => (
                <div
                  key={day.date}
                  className="flex flex-1 flex-col items-center gap-1"
                >
                  <span className="text-xs font-medium text-foreground">
                    {day.count}
                  </span>
                  <div
                    className="w-full rounded-t bg-blue-500 transition-all"
                    style={{
                      height: `${(day.count / maxDaily) * 100}%`,
                      minHeight: day.count > 0 ? "4px" : "0px",
                    }}
                  />
                  <span className="text-[10px] text-muted-foreground">
                    {formatDate(day.date)}
                  </span>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-xs text-muted-foreground">
              No tasks in the last 7 days
            </p>
          )}
        </div>
      </div>

      {/* Agent performance table */}
      <div className="rounded-lg border border-border bg-gray-900/50 p-5">
        <h2 className="mb-4 text-sm font-semibold text-foreground">
          Agent Performance
        </h2>
        {(data.agent_usage ?? []).length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs text-muted-foreground">
                  <th className="pb-2 font-medium">Agent</th>
                  <th className="pb-2 font-medium">Tasks Assigned</th>
                  <th className="pb-2 font-medium">Completed</th>
                  <th className="pb-2 font-medium">Failed</th>
                  <th className="pb-2 font-medium">Success Rate</th>
                </tr>
              </thead>
              <tbody>
                {(data.agent_usage ?? []).map((agent) => {
                  const total = agent.completed + agent.failed;
                  const rate = total > 0 ? agent.completed / total : 0;
                  return (
                    <tr
                      key={agent.name}
                      className="border-b border-border/50"
                    >
                      <td className="py-2 font-medium text-foreground">
                        {agent.name}
                      </td>
                      <td className="py-2 text-muted-foreground">
                        {agent.task_count}
                      </td>
                      <td className="py-2 text-green-400">
                        {agent.completed}
                      </td>
                      <td className="py-2 text-red-400">{agent.failed}</td>
                      <td className="py-2">
                        <span
                          className={cn(
                            "rounded-full px-2 py-0.5 text-xs font-medium",
                            rate > 0.8
                              ? "bg-green-500/20 text-green-400"
                              : rate > 0.5
                                ? "bg-amber-500/20 text-amber-400"
                                : "bg-red-500/20 text-red-400",
                          )}
                        >
                          {(rate * 100).toFixed(0)}%
                        </span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="text-xs text-muted-foreground">
            No agent activity recorded
          </p>
        )}
      </div>
    </div>
  );
}
