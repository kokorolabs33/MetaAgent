"use client";

import { cn } from "@/lib/utils";
import type { AgentActivityStatus } from "@/lib/types";

const statusConfig: Record<AgentActivityStatus, {
  color: string;
  label: string;
  pulse?: boolean;
  dashed?: boolean;
}> = {
  online: {
    color: "bg-green-500",
    label: "Online",
  },
  working: {
    color: "bg-amber-500",
    label: "Working",
    pulse: true,
  },
  idle: {
    color: "bg-green-500",
    label: "Idle",
  },
  offline: {
    color: "bg-gray-500",
    label: "Offline",
  },
  unknown: {
    color: "bg-gray-400",
    label: "Unknown",
    dashed: true,
  },
};

interface AgentStatusDotProps {
  status: AgentActivityStatus;
  size?: "sm" | "md";
  className?: string;
}

export function AgentStatusDot({ status, size = "sm", className }: AgentStatusDotProps) {
  const config = statusConfig[status] ?? statusConfig.unknown;
  const sizeClass = size === "sm" ? "size-2" : "size-2.5";

  return (
    <span
      className={cn(
        "inline-block shrink-0 rounded-full",
        sizeClass,
        config.dashed
          ? "border border-dashed border-gray-500"
          : config.color,
        config.pulse && "animate-pulse",
        className,
      )}
      title={config.label}
      aria-label={`Agent status: ${config.label}`}
    />
  );
}
