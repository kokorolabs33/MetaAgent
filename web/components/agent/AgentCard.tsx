"use client";

import { useRouter } from "next/navigation";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { Agent } from "@/lib/types";

const statusConfig: Record<
  Agent["status"],
  { label: string; className: string }
> = {
  active: {
    label: "Active",
    className: "bg-green-500/20 text-green-400",
  },
  inactive: {
    label: "Inactive",
    className: "bg-gray-500/20 text-gray-400",
  },
  degraded: {
    label: "Degraded",
    className: "bg-amber-500/20 text-amber-400",
  },
};

interface AgentCardProps {
  agent: Agent;
}

export function AgentCard({ agent }: AgentCardProps) {
  const router = useRouter();
  const status = statusConfig[agent.status];

  return (
    <Card
      className="cursor-pointer transition-colors hover:bg-card/80"
      onClick={() => router.push(`/manage/agents/${agent.id}`)}
    >
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <CardTitle className="truncate">{agent.name}</CardTitle>
            {agent.version && (
              <Badge variant="outline" className="text-xs">
                v{agent.version}
              </Badge>
            )}
          </div>
          <Badge className={status.className}>{status.label}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-2">
          {agent.description && (
            <p className="line-clamp-2 text-xs text-muted-foreground">
              {agent.description}
            </p>
          )}
          <div className="flex items-center justify-between gap-2">
            <div className="flex flex-wrap gap-1">
              {(agent.capabilities ?? []).slice(0, 4).map((cap) => (
                <span
                  key={cap}
                  className="rounded-md bg-secondary/60 px-1.5 py-0.5 text-[10px] text-muted-foreground"
                >
                  {cap}
                </span>
              ))}
              {(agent.capabilities ?? []).length > 4 && (
                <span className="rounded-md bg-secondary/60 px-1.5 py-0.5 text-[10px] text-muted-foreground">
                  +{(agent.capabilities ?? []).length - 4}
                </span>
              )}
            </div>
            <span className="shrink-0 truncate text-xs text-muted-foreground max-w-[140px]">
              {agent.endpoint}
            </span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
