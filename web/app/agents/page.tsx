"use client";

import { useEffect, useMemo } from "react";
import Link from "next/link";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AgentCard } from "@/components/agent/AgentCard";
import { useOrgStore, useAgentStore } from "@/lib/store";

export default function AgentsPage() {
  const { orgs, loadOrgs, isLoading: orgsLoading } = useOrgStore();
  const { agents, loadAgents, isLoading: agentsLoading } = useAgentStore();

  const orgId = useMemo(
    () => (orgs.length > 0 ? orgs[0].id : null),
    [orgs],
  );

  // Load orgs on mount
  useEffect(() => {
    if (orgs.length === 0) {
      loadOrgs();
    }
  }, [orgs.length, loadOrgs]);

  // Load agents when org is available
  useEffect(() => {
    if (orgId) {
      loadAgents(orgId);
    }
  }, [orgId, loadAgents]);

  const isLoading = orgsLoading || agentsLoading;

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-border px-6 py-4">
        <h1 className="text-lg font-semibold text-foreground">Agents</h1>
        <Link href="/agents/register">
          <Button>
            <Plus className="size-4" />
            Register Agent
          </Button>
        </Link>
      </div>

      {/* Agent grid */}
      <div className="flex-1 overflow-auto p-6">
        {isLoading ? (
          <div className="flex h-64 items-center justify-center">
            <p className="text-sm text-muted-foreground">Loading agents...</p>
          </div>
        ) : agents.length === 0 ? (
          <div className="flex h-64 items-center justify-center">
            <div className="text-center">
              <p className="text-sm text-muted-foreground">
                No agents registered. Register your first agent!
              </p>
            </div>
          </div>
        ) : (
          <div className="grid gap-3 sm:grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
            {agents.map((agent) => (
              <AgentCard key={agent.id} agent={agent} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
