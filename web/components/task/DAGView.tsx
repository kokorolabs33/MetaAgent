"use client";

import { useMemo, useCallback } from "react";
import {
  ReactFlow,
  type Node,
  type Edge,
  type NodeTypes,
  Background,
  BackgroundVariant,
  useNodesState,
  useEdgesState,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type { SubTask } from "@/lib/types";
import { SubtaskNode } from "./SubtaskNode";

const nodeTypes: NodeTypes = {
  subtask: SubtaskNode,
};

interface DAGViewProps {
  subtasks: SubTask[];
  onNodeClick?: (subtaskId: string) => void;
}

/**
 * Simple layered layout algorithm:
 * 1. Find subtasks with no depends_on -> layer 0
 * 2. Each subsequent layer = max(dependency layers) + 1
 * 3. Position: x = layer * 280, y = index_within_layer * 120
 */
function layoutSubtasks(subtasks: SubTask[]): { nodes: Node[]; edges: Edge[] } {
  if (subtasks.length === 0) return { nodes: [], edges: [] };

  // Build dependency map
  const layerMap = new Map<string, number>();
  const subtaskMap = new Map<string, SubTask>();
  for (const st of subtasks) {
    subtaskMap.set(st.id, st);
  }

  // Compute layer for each subtask via topological ordering
  function getLayer(id: string, visited: Set<string>): number {
    if (layerMap.has(id)) return layerMap.get(id)!;
    if (visited.has(id)) return 0; // cycle guard
    visited.add(id);

    const st = subtaskMap.get(id);
    if (!st || st.depends_on.length === 0) {
      layerMap.set(id, 0);
      return 0;
    }

    let maxDepLayer = 0;
    for (const depId of st.depends_on) {
      const depLayer = getLayer(depId, visited);
      if (depLayer + 1 > maxDepLayer) {
        maxDepLayer = depLayer + 1;
      }
    }

    layerMap.set(id, maxDepLayer);
    return maxDepLayer;
  }

  for (const st of subtasks) {
    getLayer(st.id, new Set<string>());
  }

  // Group by layer
  const layers = new Map<number, SubTask[]>();
  for (const st of subtasks) {
    const layer = layerMap.get(st.id) ?? 0;
    const group = layers.get(layer) ?? [];
    group.push(st);
    layers.set(layer, group);
  }

  // Build nodes
  const nodes: Node[] = [];
  for (const [layer, group] of layers.entries()) {
    for (let i = 0; i < group.length; i++) {
      const st = group[i];
      nodes.push({
        id: st.id,
        type: "subtask",
        position: { x: layer * 280, y: i * 130 },
        data: {
          label: st.id,
          agentName: st.agent_id,
          instruction: st.instruction,
          status: st.status,
        },
      });
    }
  }

  // Build edges
  const edges: Edge[] = [];
  for (const st of subtasks) {
    for (const depId of st.depends_on) {
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

export function DAGView({ subtasks, onNodeClick }: DAGViewProps) {
  const { nodes: initialNodes, edges: initialEdges } = useMemo(
    () => layoutSubtasks(subtasks),
    [subtasks],
  );

  const [nodes, , onNodesChange] = useNodesState(initialNodes);
  const [edges, , onEdgesChange] = useEdgesState(initialEdges);

  const handleNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      if (onNodeClick) {
        onNodeClick(node.id);
      }
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
    <div className="h-full w-full">
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
    </div>
  );
}
