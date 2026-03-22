"use client";

import { useTaskHubStore } from "@/lib/store";

const statusColor: Record<string, string> = {
  pending: "bg-yellow-500",
  running: "bg-blue-500 animate-pulse",
  completed: "bg-green-500",
  failed: "bg-red-500",
};

export function TaskList() {
  const { tasks, currentTask, selectTask } = useTaskHubStore();

  if (tasks.length === 0) return null;

  return (
    <div className="border-b border-gray-800">
      <div className="flex items-center gap-2 px-4 py-2.5 bg-gray-900/50">
        <span className="text-xs font-medium text-gray-400 uppercase tracking-wider">
          Task History
        </span>
        <span className="text-xs text-gray-600">({tasks.length})</span>
      </div>
      <div className="max-h-36 overflow-y-auto">
        {tasks.map((task) => (
          <button
            key={task.id}
            onClick={() => selectTask(task)}
            className={`w-full text-left px-4 py-2.5 border-b border-gray-800/50 hover:bg-gray-800/50 transition-colors ${
              currentTask?.id === task.id ? "bg-gray-800" : ""
            }`}
          >
            <div className="flex items-center gap-2">
              <span
                className={`w-2 h-2 rounded-full flex-shrink-0 ${statusColor[task.status] ?? "bg-gray-500"}`}
              />
              <span className="text-xs text-gray-300 truncate">{task.title || task.description}</span>
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}
