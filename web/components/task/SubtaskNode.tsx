"use client";

import { memo } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import {
  Clock,
  Loader2,
  CheckCircle2,
  XCircle,
  AlertCircle,
  Lock,
} from "lucide-react";
import { cn } from "@/lib/utils";

interface SubtaskNodeData {
  label: string;
  agentName: string;
  instruction: string;
  status: string;
  [key: string]: unknown;
}

const statusConfig: Record<
  string,
  { bg: string; border: string; icon: React.ElementType; iconClass: string }
> = {
  pending: {
    bg: "bg-gray-800/80",
    border: "border-gray-600",
    icon: Clock,
    iconClass: "text-gray-400",
  },
  running: {
    bg: "bg-blue-950/80",
    border: "border-blue-500",
    icon: Loader2,
    iconClass: "text-blue-400 animate-spin",
  },
  completed: {
    bg: "bg-green-950/80",
    border: "border-green-500",
    icon: CheckCircle2,
    iconClass: "text-green-400",
  },
  failed: {
    bg: "bg-red-950/80",
    border: "border-red-500",
    icon: XCircle,
    iconClass: "text-red-400",
  },
  input_required: {
    bg: "bg-amber-950/80",
    border: "border-amber-500",
    icon: AlertCircle,
    iconClass: "text-amber-400 animate-pulse",
  },
  blocked: {
    bg: "bg-gray-800/80",
    border: "border-gray-500",
    icon: Lock,
    iconClass: "text-gray-400",
  },
  cancelled: {
    bg: "bg-gray-800/80",
    border: "border-gray-600",
    icon: XCircle,
    iconClass: "text-gray-500",
  },
};

function SubtaskNodeComponent({ data }: NodeProps) {
  const nodeData = data as SubtaskNodeData;
  const config = statusConfig[nodeData.status] ?? statusConfig.pending;
  const Icon = config.icon;

  const truncated =
    nodeData.instruction.length > 60
      ? nodeData.instruction.slice(0, 57) + "..."
      : nodeData.instruction;

  return (
    <>
      <Handle type="target" position={Position.Left} className="!bg-gray-500" />
      <div
        className={cn(
          "rounded-lg border px-4 py-3 shadow-md min-w-[200px] max-w-[260px]",
          config.bg,
          config.border,
          nodeData.status === "input_required" && "ring-2 ring-amber-500/30",
        )}
      >
        <div className="flex items-center gap-2">
          <Icon className={cn("size-4 shrink-0", config.iconClass)} />
          <span className="truncate text-sm font-bold text-foreground">
            {nodeData.agentName}
          </span>
        </div>
        <p className="mt-1.5 text-xs leading-relaxed text-muted-foreground">
          {truncated}
        </p>
      </div>
      <Handle
        type="source"
        position={Position.Right}
        className="!bg-gray-500"
      />
    </>
  );
}

export const SubtaskNode = memo(SubtaskNodeComponent);
