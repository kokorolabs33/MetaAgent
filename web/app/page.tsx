"use client";

import { useEffect } from "react";
import { TaskBar } from "@/components/task/TaskBar";
import { AgentTopology } from "@/components/topology/AgentTopology";
import { ChannelPanel } from "@/components/channel/ChannelPanel";
import { useTaskHubStore } from "@/lib/store";

export default function Home() {
  const { loadAgents } = useTaskHubStore();

  useEffect(() => {
    loadAgents();
  }, [loadAgents]);

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      <TaskBar />
      <div className="flex flex-1 overflow-hidden">
        {/* Left: Agent Topology (40%) */}
        <div className="w-[40%] border-r border-gray-800 overflow-hidden">
          <div className="flex items-center gap-2 px-4 py-2.5 border-b border-gray-800 bg-gray-900/50">
            <span className="text-xs font-medium text-gray-400 uppercase tracking-wider">
              Agent Network
            </span>
          </div>
          <div className="h-[calc(100%-41px)]">
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
