"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { WebhookConfig } from "@/lib/types";

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

export default function WebhooksPage() {
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
      prev.includes(event) ? prev.filter((e) => e !== event) : [...prev, event]
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
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b border-border px-6 py-4">
        <h1 className="text-lg font-semibold text-foreground">Webhooks</h1>
        <Button onClick={() => setShowCreate(!showCreate)}>
          {showCreate ? "Cancel" : "Create Webhook"}
        </Button>
      </div>

      <div className="flex-1 overflow-auto p-6">
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
    </div>
  );
}
