"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Zap } from "lucide-react";
import { useTaskHubStore } from "@/lib/store";
import { MasterChatModal } from "@/components/chat/MasterChatModal";

export function TaskBar() {
  const [chatOpen, setChatOpen] = useState(false);
  const { currentTask } = useTaskHubStore();

  const statusColor: Record<string, string> = {
    pending: "bg-yellow-500/20 text-yellow-400 border-yellow-500/30",
    running: "bg-blue-500/20 text-blue-400 border-blue-500/30",
    completed: "bg-green-500/20 text-green-400 border-green-500/30",
    failed: "bg-red-500/20 text-red-400 border-red-500/30",
  };

  return (
    <>
      <div className="border-b border-gray-800 bg-gray-900/80 backdrop-blur px-6 py-4">
        <div className="flex items-center gap-4 max-w-screen-xl mx-auto">
          <div className="flex items-center gap-2 mr-4">
            <div className="w-7 h-7 rounded-lg bg-purple-600 flex items-center justify-center">
              <Zap className="w-4 h-4 text-white" />
            </div>
            <span className="font-semibold text-white text-sm">TaskHub</span>
          </div>

          <button
            onClick={() => setChatOpen(true)}
            className="flex-1 text-left bg-gray-800/60 border border-gray-700 rounded-md px-4 py-2 text-gray-500 text-sm hover:border-purple-500/50 hover:text-gray-400 transition-colors"
          >
            Chat with Master Agent to plan and dispatch a task...
          </button>

          <Button
            onClick={() => setChatOpen(true)}
            className="bg-purple-600 hover:bg-purple-700 text-white min-w-[100px]"
          >
            Dispatch
          </Button>

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

      <MasterChatModal open={chatOpen} onClose={() => setChatOpen(false)} />
    </>
  );
}
