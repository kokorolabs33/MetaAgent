"use client";

import { useEffect, useMemo, useState, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import { ArrowLeft, Loader2, Trash2 } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { api } from "@/lib/api";
import { useOrgStore, useAgentStore } from "@/lib/store";
import type { Agent } from "@/lib/types";

const statusConfig: Record<
  Agent["status"],
  { label: string; className: string }
> = {
  active: { label: "Active", className: "bg-green-500/20 text-green-400" },
  inactive: { label: "Inactive", className: "bg-gray-500/20 text-gray-400" },
  degraded: { label: "Degraded", className: "bg-amber-500/20 text-amber-400" },
};

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString();
}

export default function AgentDetailPage() {
  const params = useParams();
  const router = useRouter();
  const agentId = params.id as string;

  const { orgs, loadOrgs } = useOrgStore();
  const { deleteAgent } = useAgentStore();

  const orgId = useMemo(
    () => (orgs.length > 0 ? orgs[0].id : null),
    [orgs],
  );

  const [agent, setAgent] = useState<Agent | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isDeleting, setIsDeleting] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load orgs if needed
  useEffect(() => {
    if (orgs.length === 0) {
      loadOrgs();
    }
  }, [orgs.length, loadOrgs]);

  // Load agent detail
  useEffect(() => {
    if (!orgId) return;

    const load = async () => {
      setIsLoading(true);
      setError(null);
      try {
        const data = await api.agents.get(orgId, agentId);
        setAgent(data);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Failed to load agent",
        );
      } finally {
        setIsLoading(false);
      }
    };

    void load();
  }, [orgId, agentId]);

  const handleDelete = useCallback(async () => {
    if (!orgId || isDeleting) return;
    setIsDeleting(true);
    try {
      await deleteAgent(orgId, agentId);
      router.push("/agents");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to delete agent",
      );
      setIsDeleting(false);
      setShowDeleteConfirm(false);
    }
  }, [orgId, agentId, deleteAgent, router, isDeleting]);

  if (isLoading || !agent) {
    return (
      <div className="flex h-full items-center justify-center">
        {error ? (
          <div className="text-center">
            <p className="text-sm text-destructive">{error}</p>
            <Link
              href="/agents"
              className="mt-2 inline-block text-sm text-muted-foreground underline"
            >
              Back to Agents
            </Link>
          </div>
        ) : (
          <Loader2 className="size-6 animate-spin text-muted-foreground" />
        )}
      </div>
    );
  }

  const status = statusConfig[agent.status];

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <div className="flex items-center gap-3">
          <Link
            href="/agents"
            className="rounded-md p-1 text-muted-foreground transition-colors hover:text-foreground"
          >
            <ArrowLeft className="size-5" />
          </Link>
          <h1 className="truncate text-lg font-semibold text-foreground">
            {agent.name}
          </h1>
          <Badge className={status.className}>{status.label}</Badge>
        </div>
        <Button
          variant="destructive"
          size="sm"
          onClick={() => setShowDeleteConfirm(true)}
        >
          <Trash2 className="size-4" />
          Delete
        </Button>
      </div>

      {/* Agent details */}
      <div className="flex-1 overflow-auto p-6">
        <div className="mx-auto max-w-2xl space-y-6">
          {error && (
            <p className="text-sm text-destructive">{error}</p>
          )}

          <DetailRow label="Name" value={agent.name} />
          <DetailRow label="Description" value={agent.description || "---"} />
          <DetailRow label="Endpoint" value={agent.endpoint} mono />
          <DetailRow
            label="Adapter Type"
            value={agent.adapter_type === "http_poll" ? "HTTP Poll" : "Native"}
          />
          <DetailRow label="Auth Type" value={agent.auth_type} />
          <DetailRow label="Version" value={agent.version || "---"} />

          <div className="space-y-1">
            <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
              Capabilities
            </span>
            <div className="flex flex-wrap gap-1.5">
              {agent.capabilities.length > 0 ? (
                agent.capabilities.map((cap) => (
                  <Badge key={cap} variant="secondary" className="text-xs">
                    {cap}
                  </Badge>
                ))
              ) : (
                <span className="text-sm text-muted-foreground">None</span>
              )}
            </div>
          </div>

          <DetailRow label="Created" value={formatDate(agent.created_at)} />
          <DetailRow label="Updated" value={formatDate(agent.updated_at)} />
        </div>
      </div>

      {/* Delete confirmation modal */}
      {showDeleteConfirm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div
            className="absolute inset-0 bg-black/60"
            onClick={() => setShowDeleteConfirm(false)}
          />
          <div className="relative w-full max-w-sm rounded-xl border border-border bg-card p-6 shadow-xl">
            <h2 className="text-lg font-semibold text-card-foreground">
              Delete Agent
            </h2>
            <p className="mt-2 text-sm text-muted-foreground">
              Are you sure you want to delete{" "}
              <span className="font-medium text-foreground">{agent.name}</span>?
              This action cannot be undone.
            </p>
            <div className="mt-4 flex justify-end gap-2">
              <Button
                variant="ghost"
                onClick={() => setShowDeleteConfirm(false)}
              >
                Cancel
              </Button>
              <Button
                variant="destructive"
                onClick={() => void handleDelete()}
                disabled={isDeleting}
              >
                {isDeleting ? "Deleting..." : "Delete"}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function DetailRow({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div className="space-y-1">
      <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
        {label}
      </span>
      <p
        className={`text-sm text-foreground ${mono ? "font-mono" : ""}`}
      >
        {value}
      </p>
    </div>
  );
}
