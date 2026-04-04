"use client";

import { useMemo, useCallback, useEffect } from "react";
import {
  ReactFlow,
  type Node,
  type Edge,
  type NodeTypes,
  Background,
  BackgroundVariant,
  useNodesState,
  useEdgesState,
  useReactFlow,
  ReactFlowProvider,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type { SubTask } from "@/lib/types";
import { SubtaskNode } from "./SubtaskNode";

const nodeTypes: NodeTypes = {
  subtask: SubtaskNode,
};

interface DAGViewProps {
  subtasks: SubTask[];
  agentNames?: Record<string, string>;
  onNodeClick?: (subtaskId: string) => void;
}

function layoutSubtasks(
  subtasks: SubTask[],
  agentNames?: Record<string, string>,
): { nodes: Node[]; edges: Edge[] } {
  if (subtasks.length === 0) return { nodes: [], edges: [] };

  const layerMap = new Map<string, number>();
  const subtaskMap = new Map<string, SubTask>();
  for (const st of subtasks) {
    subtaskMap.set(st.id, st);
  }

  function getLayer(id: string, visited: Set<string>): number {
    if (layerMap.has(id)) return layerMap.get(id)!;
    if (visited.has(id)) return 0;
    visited.add(id);
    const st = subtaskMap.get(id);
    if (!st || (st.depends_on ?? []).length === 0) {
      layerMap.set(id, 0);
      return 0;
    }
    let maxDepLayer = 0;
    for (const depId of st.depends_on ?? []) {
      const depLayer = getLayer(depId, visited);
      if (depLayer + 1 > maxDepLayer) maxDepLayer = depLayer + 1;
    }
    layerMap.set(id, maxDepLayer);
    return maxDepLayer;
  }

  for (const st of subtasks) {
    getLayer(st.id, new Set<string>());
  }

  const layers = new Map<number, SubTask[]>();
  for (const st of subtasks) {
    const layer = layerMap.get(st.id) ?? 0;
    const group = layers.get(layer) ?? [];
    group.push(st);
    layers.set(layer, group);
  }

  const nodes: Node[] = [];
  for (const [layer, group] of layers.entries()) {
    for (let i = 0; i < group.length; i++) {
      const st = group[i];
      nodes.push({
        id: st.id,
        type: "subtask",
        position: { x: layer * 300, y: i * 140 },
        data: {
          label: st.id,
          agentName: agentNames?.[st.agent_id] ?? st.agent_id,
          instruction: st.instruction,
          status: st.status,
        },
      });
    }
  }

  const edges: Edge[] = [];
  for (const st of subtasks) {
    for (const depId of st.depends_on ?? []) {
      edges.push({
        id: `${depId}->${st.id}`,
        source: depId,
        target: st.id,
        animated: st.status === "running",
        style: { stroke: "#6b7280" },
      });
    }
  }

  return { nodes, edges };
}

function DAGViewInner({ subtasks, agentNames, onNodeClick }: DAGViewProps) {
  const { nodes: layoutNodes, edges: layoutEdges } = useMemo(
    () => layoutSubtasks(subtasks, agentNames),
    [subtasks, agentNames],
  );

  const [nodes, setNodes, onNodesChange] = useNodesState(layoutNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(layoutEdges);
  const { fitView } = useReactFlow();

  // Sync nodes/edges when subtasks or agentNames change
  useEffect(() => {
    setNodes(layoutNodes);
    setEdges(layoutEdges);
  }, [layoutNodes, layoutEdges, setNodes, setEdges]);

  // Fit view when nodes first appear (from 0 to N)
  const nodeCount = subtasks.length;
  useEffect(() => {
    if (nodeCount > 0) {
      // Small delay to let ReactFlow measure the nodes
      const t = setTimeout(() => fitView({ padding: 0.2 }), 100);
      return () => clearTimeout(t);
    }
  }, [nodeCount, fitView]);

  const handleNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      if (onNodeClick) onNodeClick(node.id);
    },
    [onNodeClick],
  );

  if (subtasks.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        No subtasks yet
      </div>
    );
  }

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onNodeClick={handleNodeClick}
      nodeTypes={nodeTypes}
      fitView
      minZoom={0.3}
      maxZoom={1.5}
      proOptions={{ hideAttribution: true }}
    >
      <Background variant={BackgroundVariant.Dots} color="#374151" gap={20} />
    </ReactFlow>
  );
}

export function DAGView(props: DAGViewProps) {
  return (
    <div className="h-full w-full">
      <ReactFlowProvider>
        <DAGViewInner {...props} />
      </ReactFlowProvider>
    </div>
  );
}
