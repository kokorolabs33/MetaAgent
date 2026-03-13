"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Loader2, Zap } from "lucide-react";
import { useTaskHubStore } from "@/lib/store";

export function TaskBar() {
  const [input, setInput] = useState("");
  const { currentTask, isLoading, submitTask } = useTaskHubStore();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || isLoading) return;
    await submitTask(input.trim());
    setInput("");
  };

  const statusColor: Record<string, string> = {
    pending: "bg-yellow-500/20 text-yellow-400 border-yellow-500/30",
    running: "bg-blue-500/20 text-blue-400 border-blue-500/30",
    completed: "bg-green-500/20 text-green-400 border-green-500/30",
    failed: "bg-red-500/20 text-red-400 border-red-500/30",
  };

  return (
    <div className="border-b border-gray-800 bg-gray-900/80 backdrop-blur px-6 py-4">
      <div className="flex items-center gap-4 max-w-screen-xl mx-auto">
        <div className="flex items-center gap-2 mr-4">
          <div className="w-7 h-7 rounded-lg bg-purple-600 flex items-center justify-center">
            <Zap className="w-4 h-4 text-white" />
          </div>
          <span className="font-semibold text-white text-sm">TaskHub</span>
        </div>

        <form onSubmit={handleSubmit} className="flex gap-3 flex-1">
          <Input
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Describe a task for your agents... (e.g. 'Production API is down, investigate and respond')"
            className="flex-1 bg-gray-800/60 border-gray-700 text-gray-100 placeholder:text-gray-500 focus:border-purple-500"
            disabled={isLoading}
          />
          <Button
            type="submit"
            disabled={!input.trim() || isLoading}
            className="bg-purple-600 hover:bg-purple-700 text-white min-w-[100px]"
          >
            {isLoading ? (
              <>
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                Running
              </>
            ) : (
              "Dispatch"
            )}
          </Button>
        </form>

        {currentTask && (
          <div className="flex items-center gap-2 ml-2 shrink-0">
            <span className="text-xs text-gray-400 max-w-[200px] truncate">
              {currentTask.title}
            </span>
            <Badge
              className={`text-xs border ${statusColor[currentTask.status] ?? ""}`}
            >
              {currentTask.status}
            </Badge>
          </div>
        )}
      </div>
    </div>
  );
}
