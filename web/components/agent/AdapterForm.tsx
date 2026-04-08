"use client";

import { useState, useCallback } from "react";
import { Loader2, CheckCircle, XCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import type { Agent, DiscoveredAgent } from "@/lib/types";

interface AgentDiscoveryFormProps {
  value: { endpoint?: string };
  onChange: (data: Partial<Agent>) => void;
}

export function AgentDiscoveryForm({ value, onChange }: AgentDiscoveryFormProps) {
  const [isDiscovering, setIsDiscovering] = useState(false);
  const [discovered, setDiscovered] = useState<DiscoveredAgent | null>(null);
  const [discoverError, setDiscoverError] = useState<string | null>(null);

  const handleDiscover = useCallback(async () => {
    const url = value.endpoint?.trim();
    if (!url) return;

    setIsDiscovering(true);
    setDiscoverError(null);
    setDiscovered(null);

    try {
      const { api } = await import("@/lib/api");

      const result = await api.agents.discover(url);
      setDiscovered(result);

      onChange({
        name: result.name,
        description: result.description,
        version: result.version,
        endpoint: result.url,
        agent_card_url: result.url,
        capabilities: result.capabilities,
        skills: result.skills,
      });
    } catch (err) {
      setDiscoverError(
        err instanceof Error ? err.message : "Discovery failed",
      );
    } finally {
      setIsDiscovering(false);
    }
  }, [value.endpoint, onChange]);

  return (
    <div className="space-y-5">
      {/* Endpoint / URL */}
      <div className="space-y-2">
        <label
          htmlFor="agent-endpoint"
          className="text-sm font-medium text-card-foreground"
        >
          Agent URL <span className="text-destructive">*</span>
        </label>
        <div className="flex gap-2">
          <Input
            id="agent-endpoint"
            placeholder="https://my-agent.example.com"
            value={value.endpoint ?? ""}
            onChange={(e) => {
              onChange({ endpoint: e.target.value });
              setDiscovered(null);
              setDiscoverError(null);
            }}
            required
          />
          <Button
            type="button"
            variant="outline"
            onClick={() => void handleDiscover()}
            disabled={isDiscovering || !value.endpoint?.trim()}
          >
            {isDiscovering ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              "Discover"
            )}
          </Button>
        </div>
        <p className="text-xs text-muted-foreground">
          Enter the agent&apos;s base URL. Clicking Discover will fetch its Agent Card.
        </p>
      </div>

      {/* Discovery error */}
      {discoverError && (
        <div className="flex items-center gap-2 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          <XCircle className="size-4 shrink-0" />
          {discoverError}
        </div>
      )}

      {/* Discovered agent info */}
      {discovered && (
        <div className="rounded-lg border border-green-500/30 bg-green-950/20 p-4 space-y-3">
          <div className="flex items-center gap-2">
            <CheckCircle className="size-4 text-green-400 shrink-0" />
            <span className="text-sm font-medium text-green-400">
              Agent discovered
            </span>
          </div>

          <div className="space-y-1">
            <p className="text-sm font-semibold text-foreground">
              {discovered.name}
            </p>
            {discovered.version && (
              <p className="text-xs text-muted-foreground">
                v{discovered.version}
              </p>
            )}
            {discovered.description && (
              <p className="text-xs text-muted-foreground">
                {discovered.description}
              </p>
            )}
          </div>

          {(discovered.capabilities ?? []).length > 0 && (
            <div className="space-y-1">
              <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                Capabilities
              </p>
              <div className="flex flex-wrap gap-1">
                {(discovered.capabilities ?? []).map((cap) => (
                  <Badge key={cap} variant="secondary" className="text-xs">
                    {cap}
                  </Badge>
                ))}
              </div>
            </div>
          )}

          {discovered.skills && discovered.skills.length > 0 && (
            <div className="space-y-1">
              <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                Skills
              </p>
              <ul className="space-y-1">
                {discovered.skills.map((skill) => (
                  <li key={skill.id} className="text-xs text-muted-foreground">
                    <span className="font-medium text-foreground">
                      {skill.name}
                    </span>
                    {skill.description && ` — ${skill.description}`}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// Keep named export alias for backwards compatibility with register/page.tsx import
export { AgentDiscoveryForm as AdapterForm };
