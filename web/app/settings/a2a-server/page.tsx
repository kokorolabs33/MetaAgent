"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { A2AConfig } from "@/lib/api";
import { Button } from "@/components/ui/button";

export default function A2AServerPage() {
  const [config, setConfig] = useState<A2AConfig | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.a2aConfig.get().then(setConfig).finally(() => setLoading(false));
  }, []);

  const handleToggle = async () => {
    if (!config) return;
    const updated = await api.a2aConfig.update({ enabled: !config.enabled });
    setConfig(updated);
  };

  const handleRefresh = async () => {
    await api.a2aConfig.refreshCard();
    const updated = await api.a2aConfig.get();
    setConfig(updated);
  };

  if (loading || !config) {
    return (
      <div className="flex h-64 items-center justify-center">
        <p className="text-sm text-muted-foreground">Loading...</p>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b border-border px-6 py-4">
        <h1 className="text-lg font-semibold text-foreground">A2A Server Configuration</h1>
      </div>

      <div className="flex-1 overflow-auto p-6">
        <div className="mb-6 rounded-lg border border-border bg-card p-4">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="font-semibold text-card-foreground">A2A Server</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                When enabled, MetaAgent exposes itself as an A2A agent
              </p>
            </div>
            <Button
              onClick={handleToggle}
              variant={config.enabled ? "default" : "secondary"}
            >
              {config.enabled ? "Enabled" : "Disabled"}
            </Button>
          </div>
        </div>

        <div className="mb-6">
          <div className="mb-2 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-foreground">Aggregated AgentCard</h2>
            <Button variant="secondary" size="sm" onClick={handleRefresh}>
              Refresh Card
            </Button>
          </div>
          <pre className="max-h-96 overflow-auto rounded-lg border border-border bg-card p-4 text-xs text-muted-foreground">
            {JSON.stringify(config.aggregated_card, null, 2)}
          </pre>
        </div>
      </div>
    </div>
  );
}
