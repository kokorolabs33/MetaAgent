"use client";

import { useState, useCallback } from "react";
import { Loader2, CheckCircle, XCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { Agent } from "@/lib/types";

interface AdapterFormProps {
  value: Partial<Agent>;
  onChange: (data: Partial<Agent>) => void;
}

type AuthType = Agent["auth_type"];
type AdapterType = Agent["adapter_type"];

interface HealthResult {
  ok: boolean;
  message: string;
}

export function AdapterForm({ value, onChange }: AdapterFormProps) {
  const [testResult, setTestResult] = useState<HealthResult | null>(null);
  const [isTesting, setIsTesting] = useState(false);

  // Auth credentials are stored in adapter_config for now
  const adapterCfg = (value.adapter_config ?? {}) as Record<string, unknown>;

  const updateField = useCallback(
    (field: keyof Agent, val: unknown) => {
      onChange({ ...value, [field]: val });
    },
    [value, onChange],
  );

  const updateAdapterConfig = useCallback(
    (key: string, val: unknown) => {
      const cfg = { ...(value.adapter_config ?? {}), [key]: val };
      onChange({ ...value, adapter_config: cfg });
    },
    [value, onChange],
  );

  const handleTestConnection = useCallback(async () => {
    const endpoint = value.endpoint?.trim();
    if (!endpoint) {
      setTestResult({ ok: false, message: "Endpoint is required" });
      return;
    }

    setIsTesting(true);
    setTestResult(null);
    try {
      if (value.id) {
        // Agent already saved — use the backend healthcheck API
        const { api } = await import("@/lib/api");
        const { useOrgStore } = await import("@/lib/store");
        const orgId = useOrgStore.getState().currentOrg?.id;
        if (!orgId) {
          setTestResult({ ok: false, message: "No organization selected" });
          return;
        }
        const result = await api.agents.healthcheck(orgId, value.id);
        setTestResult({
          ok: result.status >= 200 && result.status < 300,
          message: `Status ${result.status} (${result.latency_ms}ms)`,
        });
      } else {
        // Agent not saved yet — test via backend proxy
        const { api } = await import("@/lib/api");
        const { useOrgStore } = await import("@/lib/store");
        const orgId = useOrgStore.getState().currentOrg?.id;
        if (!orgId) {
          setTestResult({ ok: false, message: "No organization selected" });
          return;
        }
        const result = await api.agents.testEndpoint(orgId, endpoint);
        setTestResult({
          ok: result.status >= 200 && result.status < 300,
          message: `Status ${result.status} (${result.latency_ms}ms)`,
        });
      }
    } catch (err) {
      setTestResult({
        ok: false,
        message: err instanceof Error ? err.message : "Connection failed",
      });
    } finally {
      setIsTesting(false);
    }
  }, [value.id, value.endpoint]);

  const authType: AuthType = value.auth_type ?? "none";
  const adapterType: AdapterType = value.adapter_type ?? "native";

  return (
    <div className="space-y-5">
      {/* Name */}
      <div className="space-y-2">
        <label
          htmlFor="agent-name"
          className="text-sm font-medium text-card-foreground"
        >
          Name <span className="text-destructive">*</span>
        </label>
        <Input
          id="agent-name"
          placeholder="My Agent"
          value={value.name ?? ""}
          onChange={(e) => updateField("name", e.target.value)}
          required
        />
      </div>

      {/* Description */}
      <div className="space-y-2">
        <label
          htmlFor="agent-description"
          className="text-sm font-medium text-card-foreground"
        >
          Description
        </label>
        <textarea
          id="agent-description"
          placeholder="What does this agent do?"
          value={value.description ?? ""}
          onChange={(e) => updateField("description", e.target.value)}
          rows={3}
          className="w-full rounded-lg border border-input bg-transparent px-2.5 py-2 text-sm transition-colors placeholder:text-muted-foreground focus-visible:border-ring focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
        />
      </div>

      {/* Capabilities */}
      <div className="space-y-2">
        <label
          htmlFor="agent-capabilities"
          className="text-sm font-medium text-card-foreground"
        >
          Capabilities
        </label>
        <Input
          id="agent-capabilities"
          placeholder="code-review, testing, deployment"
          value={(value.capabilities ?? []).join(", ")}
          onChange={(e) =>
            updateField(
              "capabilities",
              e.target.value
                .split(",")
                .map((s) => s.trim())
                .filter(Boolean),
            )
          }
        />
        <p className="text-xs text-muted-foreground">
          Comma-separated list of capabilities
        </p>
      </div>

      {/* Endpoint */}
      <div className="space-y-2">
        <label
          htmlFor="agent-endpoint"
          className="text-sm font-medium text-card-foreground"
        >
          Endpoint <span className="text-destructive">*</span>
        </label>
        <Input
          id="agent-endpoint"
          placeholder="https://my-agent.example.com/api"
          value={value.endpoint ?? ""}
          onChange={(e) => updateField("endpoint", e.target.value)}
          required
        />
      </div>

      {/* Adapter Type */}
      <div className="space-y-2">
        <label
          htmlFor="agent-adapter-type"
          className="text-sm font-medium text-card-foreground"
        >
          Adapter Type
        </label>
        <select
          id="agent-adapter-type"
          value={adapterType}
          onChange={(e) =>
            updateField("adapter_type", e.target.value as AdapterType)
          }
          className="w-full rounded-lg border border-input bg-transparent px-2.5 py-1.5 text-sm transition-colors focus-visible:border-ring focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
        >
          <option value="native" className="bg-card">
            Native
          </option>
          <option value="http_poll" className="bg-card">
            HTTP Poll
          </option>
        </select>
      </div>

      {/* Auth Type */}
      <div className="space-y-2">
        <label
          htmlFor="agent-auth-type"
          className="text-sm font-medium text-card-foreground"
        >
          Auth Type
        </label>
        <select
          id="agent-auth-type"
          value={authType}
          onChange={(e) =>
            updateField("auth_type", e.target.value as AuthType)
          }
          className="w-full rounded-lg border border-input bg-transparent px-2.5 py-1.5 text-sm transition-colors focus-visible:border-ring focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
        >
          <option value="none" className="bg-card">
            None
          </option>
          <option value="bearer" className="bg-card">
            Bearer Token
          </option>
          <option value="api_key" className="bg-card">
            API Key
          </option>
          <option value="basic" className="bg-card">
            Basic Auth
          </option>
        </select>
      </div>

      {/* Conditional auth fields */}
      {authType === "bearer" && (
        <div className="space-y-2">
          <label
            htmlFor="auth-token"
            className="text-sm font-medium text-card-foreground"
          >
            Bearer Token
          </label>
          <Input
            id="auth-token"
            type="password"
            placeholder="Token"
            value={(adapterCfg.bearer_token as string) ?? ""}
            onChange={(e) => updateAdapterConfig("bearer_token", e.target.value)}
          />
        </div>
      )}

      {authType === "api_key" && (
        <div className="space-y-4">
          <div className="space-y-2">
            <label
              htmlFor="auth-api-key"
              className="text-sm font-medium text-card-foreground"
            >
              API Key
            </label>
            <Input
              id="auth-api-key"
              type="password"
              placeholder="Key"
              value={(adapterCfg.api_key as string) ?? ""}
              onChange={(e) => updateAdapterConfig("api_key", e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <label
              htmlFor="auth-header"
              className="text-sm font-medium text-card-foreground"
            >
              Header Name
            </label>
            <Input
              id="auth-header"
              placeholder="X-API-Key"
              value={(adapterCfg.api_key_header as string) ?? ""}
              onChange={(e) =>
                updateAdapterConfig("api_key_header", e.target.value)
              }
            />
          </div>
        </div>
      )}

      {authType === "basic" && (
        <div className="space-y-4">
          <div className="space-y-2">
            <label
              htmlFor="auth-username"
              className="text-sm font-medium text-card-foreground"
            >
              Username
            </label>
            <Input
              id="auth-username"
              placeholder="Username"
              value={(adapterCfg.basic_username as string) ?? ""}
              onChange={(e) =>
                updateAdapterConfig("basic_username", e.target.value)
              }
            />
          </div>
          <div className="space-y-2">
            <label
              htmlFor="auth-password"
              className="text-sm font-medium text-card-foreground"
            >
              Password
            </label>
            <Input
              id="auth-password"
              type="password"
              placeholder="Password"
              value={(adapterCfg.basic_password as string) ?? ""}
              onChange={(e) =>
                updateAdapterConfig("basic_password", e.target.value)
              }
            />
          </div>
        </div>
      )}

      {/* Adapter config JSON (for http_poll) */}
      {adapterType === "http_poll" && (
        <div className="space-y-2">
          <label
            htmlFor="adapter-config"
            className="text-sm font-medium text-card-foreground"
          >
            Adapter Config (JSON)
          </label>
          <textarea
            id="adapter-config"
            placeholder='{"poll_interval_seconds": 10}'
            value={JSON.stringify(value.adapter_config ?? {}, null, 2)}
            onChange={(e) => {
              try {
                const parsed = JSON.parse(e.target.value) as Record<
                  string,
                  unknown
                >;
                onChange({ ...value, adapter_config: parsed });
              } catch {
                // Allow user to keep typing; only update on valid JSON
              }
            }}
            rows={4}
            className="w-full rounded-lg border border-input bg-transparent px-2.5 py-2 font-mono text-xs transition-colors placeholder:text-muted-foreground focus-visible:border-ring focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
          />
          <p className="text-xs text-muted-foreground">
            Advanced: raw JSON configuration for the HTTP poll adapter
          </p>
        </div>
      )}

      {/* Test Connection */}
      <div className="flex items-center gap-3">
        <Button
          type="button"
          variant="outline"
          onClick={() => void handleTestConnection()}
          disabled={isTesting || !value.endpoint?.trim()}
        >
          {isTesting ? (
            <Loader2 className="size-4 animate-spin" />
          ) : (
            "Test Connection"
          )}
        </Button>
        {testResult && (
          <div className="flex items-center gap-1.5 text-sm">
            {testResult.ok ? (
              <CheckCircle className="size-4 text-green-400" />
            ) : (
              <XCircle className="size-4 text-red-400" />
            )}
            <span
              className={
                testResult.ok ? "text-green-400" : "text-red-400"
              }
            >
              {testResult.message}
            </span>
          </div>
        )}
      </div>
    </div>
  );
}
