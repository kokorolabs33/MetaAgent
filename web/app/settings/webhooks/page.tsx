"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { WebhookConfig, InboundWebhook } from "@/lib/types";

// ─── Outbound webhook constants ───

const WEBHOOK_EVENTS = [
  "task.completed",
  "task.failed",
  "task.cancelled",
  "subtask.completed",
  "subtask.failed",
  "approval.requested",
  "approval.resolved",
  "agent.offline",
  "agent.skill_drift",
] as const;

// ─── Provider helpers ───

const PROVIDERS = ["github", "slack", "generic"] as const;
type Provider = (typeof PROVIDERS)[number];

const PROVIDER_LABELS: Record<Provider, string> = {
  github: "GitHub",
  slack: "Slack",
  generic: "Generic",
};

const PROVIDER_BADGE_CLASSES: Record<Provider, string> = {
  github: "bg-gray-800 text-gray-200",
  slack: "bg-purple-900 text-purple-200",
  generic: "bg-blue-900 text-blue-200",
};

const PROVIDER_INSTRUCTIONS: Record<Provider, string> = {
  github:
    "Go to your repository Settings > Webhooks > Add webhook. Paste the URL above as the Payload URL. Set Content type to application/json. Paste the secret above. Select 'Push events' and/or 'Pull requests' for events.",
  slack:
    "Go to api.slack.com > Your App > Slash Commands > Create New Command. Paste the URL above as the Request URL. Or use Event Subscriptions and paste the URL as the Request URL.",
  generic:
    'Send POST requests to the URL above with JSON body {"title": "...", "description": "..."}. Include the HMAC-SHA256 signature in the X-Webhook-Signature header.',
};

// ─── Outbound Tab ───

function OutboundTab() {
  const [webhooks, setWebhooks] = useState<WebhookConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState("");
  const [newUrl, setNewUrl] = useState("");
  const [newSecret, setNewSecret] = useState("");
  const [newEvents, setNewEvents] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [testingId, setTestingId] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<string | null>(null);

  useEffect(() => {
    api.webhooks.list().then(setWebhooks).finally(() => setLoading(false));
  }, []);

  const toggleEvent = (event: string) => {
    setNewEvents((prev) =>
      prev.includes(event) ? prev.filter((e) => e !== event) : [...prev, event],
    );
  };

  const handleCreate = async () => {
    setError(null);
    if (!newName.trim() || !newUrl.trim()) {
      setError("Name and URL are required");
      return;
    }
    if (newEvents.length === 0) {
      setError("Select at least one event");
      return;
    }
    try {
      const w = await api.webhooks.create({
        name: newName,
        url: newUrl,
        events: newEvents,
        secret: newSecret || undefined,
      });
      setWebhooks((prev) => [w, ...prev]);
      setShowCreate(false);
      setNewName("");
      setNewUrl("");
      setNewSecret("");
      setNewEvents([]);
    } catch {
      setError("Failed to create webhook");
    }
  };

  const handleDelete = async (id: string) => {
    await api.webhooks.delete(id);
    setWebhooks((prev) => prev.filter((w) => w.id !== id));
  };

  const handleToggle = async (id: string, isActive: boolean) => {
    const updated = await api.webhooks.update(id, { is_active: !isActive });
    setWebhooks((prev) => prev.map((w) => (w.id === id ? updated : w)));
  };

  const handleTest = async (id: string) => {
    setTestingId(id);
    setTestResult(null);
    try {
      const result = await api.webhooks.test(id);
      if (result.success) {
        setTestResult(`Success (HTTP ${result.status_code})`);
      } else {
        setTestResult(`Failed: ${result.error || `HTTP ${result.status_code}`}`);
      }
    } catch {
      setTestResult("Failed to send test");
    } finally {
      setTestingId(null);
    }
  };

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <p className="text-sm text-muted-foreground">Loading webhooks...</p>
      </div>
    );
  }

  return (
    <div>
      <div className="mb-4 flex justify-end">
        <Button onClick={() => setShowCreate(!showCreate)}>
          {showCreate ? "Cancel" : "Create Webhook"}
        </Button>
      </div>

      {showCreate && (
        <div className="mb-6 rounded-lg border border-border bg-card p-4">
          <div className="space-y-3">
            <Input
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="Webhook name"
            />
            <Input
              value={newUrl}
              onChange={(e) => setNewUrl(e.target.value)}
              placeholder="https://example.com/webhook"
            />
            <Input
              type="password"
              value={newSecret}
              onChange={(e) => setNewSecret(e.target.value)}
              placeholder="Secret (optional, for HMAC signing)"
            />

            <div>
              <label className="mb-2 block text-xs font-medium text-muted-foreground">
                Subscribe to events
              </label>
              <div className="flex flex-wrap gap-2">
                {WEBHOOK_EVENTS.map((event) => (
                  <label
                    key={event}
                    className="flex items-center gap-1.5 text-sm text-muted-foreground"
                  >
                    <input
                      type="checkbox"
                      checked={newEvents.includes(event)}
                      onChange={() => toggleEvent(event)}
                      className="rounded border-zinc-600"
                    />
                    {event}
                  </label>
                ))}
              </div>
            </div>

            {error && <p className="text-sm text-destructive">{error}</p>}
            <Button onClick={handleCreate} disabled={!newName.trim() || !newUrl.trim()}>
              Save Webhook
            </Button>
          </div>
        </div>
      )}

      {testResult && (
        <div className="mb-4 rounded-lg border border-border bg-card p-3 text-sm text-muted-foreground">
          Test result: {testResult}
        </div>
      )}

      {webhooks.length === 0 ? (
        <div className="flex h-64 items-center justify-center">
          <p className="text-sm text-muted-foreground">No webhooks configured.</p>
        </div>
      ) : (
        <div className="grid gap-3">
          {webhooks.map((w) => (
            <div key={w.id} className="rounded-lg border border-border bg-card p-4">
              <div className="flex items-start justify-between">
                <div className="flex-1">
                  <h2 className="font-semibold text-card-foreground">{w.name}</h2>
                  <p className="mt-1 font-mono text-xs text-muted-foreground">{w.url}</p>
                  <div className="mt-2 flex flex-wrap gap-1">
                    {(w.events ?? []).map((event) => (
                      <span
                        key={event}
                        className="rounded bg-blue-900/30 px-1.5 py-0.5 text-xs text-blue-400"
                      >
                        {event}
                      </span>
                    ))}
                  </div>
                  {w.secret && (
                    <p className="mt-1 text-xs text-muted-foreground">
                      Secret: {w.secret}
                    </p>
                  )}
                </div>
                <div className="ml-4 flex items-center gap-2">
                  <button
                    onClick={() => handleToggle(w.id, w.is_active)}
                    className={`rounded px-2 py-1 text-xs ${
                      w.is_active
                        ? "bg-green-900/50 text-green-400"
                        : "bg-secondary text-muted-foreground"
                    }`}
                  >
                    {w.is_active ? "Active" : "Inactive"}
                  </button>
                  <button
                    onClick={() => handleTest(w.id)}
                    disabled={testingId === w.id}
                    className="rounded bg-blue-900/50 px-2 py-1 text-xs text-blue-400 transition-colors hover:bg-blue-900 disabled:opacity-50"
                  >
                    {testingId === w.id ? "Testing..." : "Test"}
                  </button>
                  <button
                    onClick={() => handleDelete(w.id)}
                    className="rounded bg-red-900/50 px-2 py-1 text-xs text-red-400 transition-colors hover:bg-red-900"
                  >
                    Delete
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ─── Copy-to-clipboard helper ───

function CopyButton({ text, label }: { text: string; label?: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback: select text in a temporary input
      const el = document.createElement("textarea");
      el.value = text;
      document.body.appendChild(el);
      el.select();
      document.execCommand("copy");
      document.body.removeChild(el);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }, [text]);

  return (
    <button
      onClick={handleCopy}
      className="rounded bg-secondary px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-secondary/80"
    >
      {copied ? "Copied!" : label ?? "Copy"}
    </button>
  );
}

// ─── Secret Reveal Card (after create or rotate) ───

function SecretRevealCard({
  webhook,
  onDone,
}: {
  webhook: InboundWebhook;
  onDone: () => void;
}) {
  const endpointUrl =
    typeof window !== "undefined"
      ? `${window.location.origin}${webhook.endpoint_url}`
      : webhook.endpoint_url;
  const provider = webhook.provider as Provider;

  return (
    <div className="mb-6 rounded-lg border-2 border-yellow-600/50 bg-yellow-950/20 p-5">
      <h3 className="mb-3 text-sm font-semibold text-yellow-400">
        Webhook Created Successfully
      </h3>

      <div className="space-y-4">
        {/* Endpoint URL */}
        <div>
          <label className="mb-1 block text-xs font-medium text-muted-foreground">
            Endpoint URL
          </label>
          <div className="flex items-center gap-2">
            <code className="flex-1 rounded bg-secondary/50 px-3 py-2 font-mono text-xs text-foreground">
              {endpointUrl}
            </code>
            <CopyButton text={endpointUrl} label="Copy URL" />
          </div>
        </div>

        {/* Secret */}
        <div>
          <label className="mb-1 block text-xs font-medium text-muted-foreground">
            Signing Secret
          </label>
          <div className="flex items-center gap-2">
            <code className="flex-1 rounded bg-secondary/50 px-3 py-2 font-mono text-xs text-foreground break-all">
              {webhook.secret}
            </code>
            <CopyButton text={webhook.secret} label="Copy Secret" />
          </div>
        </div>

        {/* Warning */}
        <div className="rounded bg-red-950/30 p-3 text-xs text-red-400">
          Save this secret now. It will not be shown again.
        </div>

        {/* Provider-specific instructions */}
        <div>
          <label className="mb-1 block text-xs font-medium text-muted-foreground">
            Setup Instructions ({PROVIDER_LABELS[provider]})
          </label>
          <p className="rounded bg-secondary/30 p-3 text-xs leading-relaxed text-muted-foreground">
            {PROVIDER_INSTRUCTIONS[provider]}
          </p>
        </div>

        <Button onClick={onDone} className="w-full">
          Done
        </Button>
      </div>
    </div>
  );
}

// ─── Inbound Tab ───

function InboundTab() {
  const [webhooks, setWebhooks] = useState<InboundWebhook[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState("");
  const [newProvider, setNewProvider] = useState<Provider>("github");
  const [error, setError] = useState<string | null>(null);
  const [revealedWebhook, setRevealedWebhook] = useState<InboundWebhook | null>(null);
  const [rotatedWebhook, setRotatedWebhook] = useState<InboundWebhook | null>(null);

  const loadWebhooks = useCallback(async () => {
    try {
      const data = await api.inboundWebhooks.list();
      setWebhooks(data);
    } catch {
      // Silently handle load failure
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadWebhooks();
  }, [loadWebhooks]);

  const handleCreate = async () => {
    setError(null);
    if (!newName.trim()) {
      setError("Name is required");
      return;
    }
    try {
      const created = await api.inboundWebhooks.create({
        name: newName.trim(),
        provider: newProvider,
      });
      setRevealedWebhook(created);
      setShowCreate(false);
      setNewName("");
      setNewProvider("github");
    } catch {
      setError("Failed to create inbound webhook");
    }
  };

  const handleRevealDone = () => {
    setRevealedWebhook(null);
    loadWebhooks();
  };

  const handleRotateDone = () => {
    setRotatedWebhook(null);
    loadWebhooks();
  };

  const handleToggle = async (id: string, isActive: boolean) => {
    // Optimistic update
    setWebhooks((prev) =>
      prev.map((w) => (w.id === id ? { ...w, is_active: !isActive } : w)),
    );
    try {
      await api.inboundWebhooks.update(id, { is_active: !isActive });
    } catch {
      // Revert on failure
      setWebhooks((prev) =>
        prev.map((w) => (w.id === id ? { ...w, is_active: isActive } : w)),
      );
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.inboundWebhooks.delete(id);
      setWebhooks((prev) => prev.filter((w) => w.id !== id));
    } catch {
      // Silently handle delete failure
    }
  };

  const handleRotateSecret = async (id: string) => {
    try {
      const updated = await api.inboundWebhooks.update(id, { rotate_secret: true });
      // After rotation, the response has masked secret -- we need the unmasked one
      // The backend returns masked secrets on update, so we fetch the newly rotated one
      // Actually, per the backend code, update always returns masked. We show a confirmation instead.
      setRotatedWebhook(updated);
    } catch {
      // Silently handle rotate failure
    }
  };

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <p className="text-sm text-muted-foreground">Loading inbound webhooks...</p>
      </div>
    );
  }

  return (
    <div>
      <div className="mb-4 flex justify-end">
        <Button onClick={() => setShowCreate(!showCreate)}>
          {showCreate ? "Cancel" : "Create Inbound Webhook"}
        </Button>
      </div>

      {/* Secret reveal after create */}
      {revealedWebhook && (
        <SecretRevealCard webhook={revealedWebhook} onDone={handleRevealDone} />
      )}

      {/* Rotated secret notification */}
      {rotatedWebhook && (
        <div className="mb-6 rounded-lg border border-yellow-600/50 bg-yellow-950/20 p-4">
          <h3 className="mb-2 text-sm font-semibold text-yellow-400">
            Secret Rotated
          </h3>
          <p className="mb-3 text-xs text-muted-foreground">
            The secret for &quot;{rotatedWebhook.name}&quot; has been rotated. The
            previous secret will continue to work temporarily during the grace period.
            Update your external service with the new secret.
          </p>
          <Button onClick={handleRotateDone} variant="outline" size="sm">
            Dismiss
          </Button>
        </div>
      )}

      {/* Create form */}
      {showCreate && (
        <div className="mb-6 rounded-lg border border-border bg-card p-4">
          <div className="space-y-3">
            <Input
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="Webhook name (e.g., My GitHub Webhook)"
            />

            <div>
              <label className="mb-2 block text-xs font-medium text-muted-foreground">
                Provider
              </label>
              <div className="flex gap-2">
                {PROVIDERS.map((p) => (
                  <button
                    key={p}
                    onClick={() => setNewProvider(p)}
                    className={`rounded px-3 py-1.5 text-sm transition-colors ${
                      newProvider === p
                        ? "bg-foreground text-background"
                        : "bg-secondary text-muted-foreground hover:bg-secondary/80"
                    }`}
                  >
                    {PROVIDER_LABELS[p]}
                  </button>
                ))}
              </div>
            </div>

            {error && <p className="text-sm text-destructive">{error}</p>}
            <Button onClick={handleCreate} disabled={!newName.trim()}>
              Create Webhook
            </Button>
          </div>
        </div>
      )}

      {/* Webhook list */}
      {webhooks.length === 0 ? (
        <div className="flex h-64 items-center justify-center">
          <p className="text-sm text-muted-foreground">
            No inbound webhooks configured. Create one to receive events from external
            services.
          </p>
        </div>
      ) : (
        <div className="grid gap-3">
          {webhooks.map((w) => {
            const provider = w.provider as Provider;
            const endpointUrl =
              typeof window !== "undefined"
                ? `${window.location.origin}${w.endpoint_url}`
                : w.endpoint_url;

            return (
              <div key={w.id} className="rounded-lg border border-border bg-card p-4">
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <h2 className="font-semibold text-card-foreground">{w.name}</h2>
                      <span
                        className={`rounded px-1.5 py-0.5 text-xs font-medium ${PROVIDER_BADGE_CLASSES[provider] ?? "bg-secondary text-muted-foreground"}`}
                      >
                        {PROVIDER_LABELS[provider] ?? w.provider}
                      </span>
                    </div>
                    <div className="mt-2 flex items-center gap-2">
                      <code className="truncate rounded bg-secondary/50 px-2 py-1 font-mono text-xs text-muted-foreground">
                        {endpointUrl}
                      </code>
                      <CopyButton text={endpointUrl} label="Copy" />
                    </div>
                  </div>
                  <div className="ml-4 flex flex-shrink-0 items-center gap-2">
                    <button
                      onClick={() => handleToggle(w.id, w.is_active)}
                      className={`rounded px-2 py-1 text-xs ${
                        w.is_active
                          ? "bg-green-900/50 text-green-400"
                          : "bg-secondary text-muted-foreground"
                      }`}
                    >
                      {w.is_active ? "Active" : "Inactive"}
                    </button>
                    <button
                      onClick={() => handleRotateSecret(w.id)}
                      className="rounded bg-yellow-900/50 px-2 py-1 text-xs text-yellow-400 transition-colors hover:bg-yellow-900"
                    >
                      Rotate Secret
                    </button>
                    <button
                      onClick={() => handleDelete(w.id)}
                      className="rounded bg-red-900/50 px-2 py-1 text-xs text-red-400 transition-colors hover:bg-red-900"
                    >
                      Delete
                    </button>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

// ─── Main Page with Tabs ───

type TabId = "outbound" | "inbound";

export default function WebhooksPage() {
  const [activeTab, setActiveTab] = useState<TabId>("outbound");

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b border-border px-6 py-4">
        <h1 className="text-lg font-semibold text-foreground">Webhooks</h1>
      </div>

      {/* Tab bar */}
      <div className="flex gap-6 border-b border-border px-6">
        {(["outbound", "inbound"] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`border-b-2 px-1 py-3 text-sm font-medium transition-colors ${
              activeTab === tab
                ? "border-foreground text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            {tab === "outbound" ? "Outbound" : "Inbound"}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div className="flex-1 overflow-auto p-6">
        {activeTab === "outbound" ? <OutboundTab /> : <InboundTab />}
      </div>
    </div>
  );
}
