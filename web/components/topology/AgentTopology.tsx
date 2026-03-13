"use client";

import { useEffect, useMemo } from "react";
import {
  ReactFlow,
  Node,
  Edge,
  Background,
  BackgroundVariant,
  useNodesState,
  useEdgesState,
  Handle,
  Position,
  NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { CheckCircle2 } from "lucide-react";
import { useTaskHubStore } from "@/lib/store";

const MASTER_ID = "master";
const MASTER_COLOR = "#8b5cf6";

function AgentNode({ data }: NodeProps) {
  const { label, color, status, ismaster } = data as {
    label: string;
    color: string;
    status: string;
    ismaster: boolean;
  };

  const isWorking = status === "working";
  const isDone = status === "done";

  return (
    <div className="relative flex flex-col items-center gap-1" style={{ minWidth: 100 }}>
      <Handle type="target" position={Position.Top} className="opacity-0" />
      <div
        className={`w-16 h-16 rounded-full flex items-center justify-center border-2 transition-all duration-300 ${
          isWorking ? "animate-pulse shadow-lg" : ""
        }`}
        style={{
          borderColor: color,
          backgroundColor: `${color}22`,
          boxShadow: isWorking ? `0 0 20px ${color}88` : undefined,
        }}
      >
        {isDone ? (
          <CheckCircle2 className="w-7 h-7 text-green-400" />
        ) : (
          <span className="text-xl font-bold" style={{ color }}>
            {label.charAt(0)}
          </span>
        )}
      </div>
      <span className="text-xs text-gray-300 text-center leading-tight max-w-[90px]">
        {label}
      </span>
      {!ismaster && status && (
        <span
          className="text-[10px] px-1.5 py-0.5 rounded-full"
          style={{
            backgroundColor: isWorking ? "#3b82f622" : isDone ? "#10b98122" : "#6b728022",
            color: isWorking ? "#60a5fa" : isDone ? "#34d399" : "#9ca3af",
          }}
        >
          {status}
        </span>
      )}
      <Handle type="source" position={Position.Bottom} className="opacity-0" />
    </div>
  );
}

const nodeTypes = { agent: AgentNode };

export function AgentTopology() {
  const { agents, channelAgents } = useTaskHubStore();

  const statusMap = useMemo(() => {
    const m: Record<string, string> = {};
    for (const ca of channelAgents) {
      m[ca.agent_id] = ca.status;
    }
    return m;
  }, [channelAgents]);

  const { nodes: initialNodes, edges: initialEdges } = useMemo(() => {
    const nodes: Node[] = [
      {
        id: MASTER_ID,
        type: "agent",
        position: { x: 200, y: 20 },
        data: { label: "Master Agent", color: MASTER_COLOR, status: "", ismaster: true },
      },
    ];
    const edges: Edge[] = [];
    const activeAgentIds = new Set(channelAgents.map((ca) => ca.agent_id));

    const displayAgents = agents.length > 0 ? agents : [];

    const cols = Math.min(Math.max(displayAgents.length, 1), 4);
    displayAgents.forEach((agent, i) => {
      const col = i % cols;
      const row = Math.floor(i / cols);
      const x = (col - (cols - 1) / 2) * 160 + 200;
      const y = 200 + row * 160;
      const status = statusMap[agent.id] ?? "idle";
      const isActive = activeAgentIds.has(agent.id);

      nodes.push({
        id: agent.id,
        type: "agent",
        position: { x, y },
        data: { label: agent.name, color: agent.color, status, ismaster: false },
        style: {
          opacity: activeAgentIds.size === 0 || isActive ? 1 : 0.35,
          transition: "opacity 0.3s",
        },
      });

      if (isActive) {
        edges.push({
          id: `master-${agent.id}`,
          source: MASTER_ID,
          target: agent.id,
          style: { stroke: agent.color, strokeWidth: 2, opacity: 0.7 },
          animated: status === "working",
        });
      }
    });

    return { nodes, edges };
  }, [agents, channelAgents, statusMap]);

  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges);

  useEffect(() => {
    setNodes(initialNodes);
    setEdges(initialEdges);
  }, [initialNodes, initialEdges, setNodes, setEdges]);

  return (
    <div className="w-full h-full bg-gray-950">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.3 }}
        minZoom={0.5}
        maxZoom={2}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        proOptions={{ hideAttribution: true }}
      >
        <Background variant={BackgroundVariant.Dots} gap={24} size={1} color="#374151" />
      </ReactFlow>
    </div>
  );
}
