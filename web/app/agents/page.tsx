"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Plus, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AgentCard } from "@/components/agent/AgentCard";
import { useAgentStore } from "@/lib/store";

export default function AgentsPage() {
  const { agents, loadAgents, isLoading, connectStatusSSE, disconnectStatusSSE } = useAgentStore();
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

  // Connect to agent status SSE for real-time status dots
  useEffect(() => {
    connectStatusSSE();
    return () => disconnectStatusSSE();
  }, [connectStatusSSE, disconnectStatusSSE]);

  // Debounce search input (300ms).
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(search), 300);
    return () => clearTimeout(timer);
  }, [search]);

  // Load agents on mount and when search changes.
  useEffect(() => {
    void loadAgents(debouncedSearch || undefined);
  }, [debouncedSearch, loadAgents]);

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-border px-6 py-4">
        <div className="flex items-center gap-4">
          <h1 className="text-lg font-semibold text-foreground">Agents</h1>
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search agents..."
              className="w-64 rounded-lg border border-input bg-transparent pl-10 pr-4 py-1.5 text-sm text-foreground placeholder:text-muted-foreground transition-colors focus-visible:border-ring focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
            />
          </div>
        </div>
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
                {debouncedSearch
                  ? "No agents match your search."
                  : "No agents registered. Register your first agent!"}
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
