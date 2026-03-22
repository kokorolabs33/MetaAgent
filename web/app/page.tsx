"use client";

import { useEffect } from "react";
import { TaskBar } from "@/components/task/TaskBar";
import { AgentTopology } from "@/components/topology/AgentTopology";
import { ChannelPanel } from "@/components/channel/ChannelPanel";
import { TaskList } from "@/components/task/TaskList";
import { useTaskHubStore } from "@/lib/store";

export default function Home() {
  const { loadAgents, loadTasks } = useTaskHubStore();

  useEffect(() => {
    loadAgents();
    loadTasks();
  }, [loadAgents, loadTasks]);

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      <TaskBar />
      <div className="flex flex-1 overflow-hidden">
        {/* Left: Task List + Agent Topology */}
        <div className="w-[40%] border-r border-gray-800 overflow-hidden flex flex-col">
          {/* Task History */}
          <TaskList />
          {/* Agent Network */}
          <div className="flex items-center gap-2 px-4 py-2.5 border-b border-t border-gray-800 bg-gray-900/50">
            <span className="text-xs font-medium text-gray-400 uppercase tracking-wider">
              Agent Network
            </span>
          </div>
          <div className="flex-1 min-h-0">
            <AgentTopology />
          </div>
        </div>

        {/* Right: Channel Panel (60%) */}
        <div className="flex-1 overflow-hidden">
          <ChannelPanel />
        </div>
      </div>
    </div>
  );
}
