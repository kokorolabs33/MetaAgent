"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { Policy } from "@/lib/types";

export default function PoliciesPage() {
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState("");
  const [newRules, setNewRules] = useState("{}");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.policies.list().then(setPolicies).finally(() => setLoading(false));
  }, []);

  const handleCreate = async () => {
    setError(null);
    try {
      const rules: Record<string, unknown> = JSON.parse(newRules);
      const p = await api.policies.create({ name: newName, rules });
      setPolicies((prev) => [p, ...prev]);
      setShowCreate(false);
      setNewName("");
      setNewRules("{}");
    } catch {
      setError("Invalid JSON in rules");
    }
  };

  const handleDelete = async (id: string) => {
    await api.policies.delete(id);
    setPolicies((prev) => prev.filter((p) => p.id !== id));
  };

  const handleToggle = async (id: string, isActive: boolean) => {
    const updated = await api.policies.update(id, { is_active: !isActive });
    setPolicies((prev) => prev.map((p) => (p.id === id ? updated : p)));
  };

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <p className="text-sm text-muted-foreground">Loading policies...</p>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b border-border px-6 py-4">
        <h1 className="text-lg font-semibold text-foreground">Policies</h1>
        <Button onClick={() => setShowCreate(!showCreate)}>
          {showCreate ? "Cancel" : "Create Policy"}
        </Button>
      </div>

      <div className="flex-1 overflow-auto p-6">
        {showCreate && (
          <div className="mb-6 rounded-lg border border-border bg-card p-4">
            <div className="space-y-3">
              <Input
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="Policy name"
              />
              <textarea
                value={newRules}
                onChange={(e) => setNewRules(e.target.value)}
                placeholder="Rules JSON"
                rows={6}
                className="w-full rounded-lg border border-input bg-transparent px-2.5 py-2 font-mono text-sm transition-colors placeholder:text-muted-foreground focus-visible:border-ring focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
              />
              {error && <p className="text-sm text-destructive">{error}</p>}
              <Button onClick={handleCreate} disabled={!newName.trim()}>
                Save Policy
              </Button>
            </div>
          </div>
        )}

        {policies.length === 0 ? (
          <div className="flex h-64 items-center justify-center">
            <p className="text-sm text-muted-foreground">No policies configured.</p>
          </div>
        ) : (
          <div className="grid gap-3">
            {policies.map((p) => (
              <div key={p.id} className="rounded-lg border border-border bg-card p-4">
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <h2 className="font-semibold text-card-foreground">{p.name}</h2>
                    <pre className="mt-2 max-h-24 overflow-auto text-xs text-muted-foreground">
                      {JSON.stringify(p.rules, null, 2)}
                    </pre>
                  </div>
                  <div className="ml-4 flex items-center gap-2">
                    <span className="text-xs text-muted-foreground">
                      Priority: {p.priority}
                    </span>
                    <button
                      onClick={() => handleToggle(p.id, p.is_active)}
                      className={`rounded px-2 py-1 text-xs ${
                        p.is_active
                          ? "bg-green-900/50 text-green-400"
                          : "bg-secondary text-muted-foreground"
                      }`}
                    >
                      {p.is_active ? "Active" : "Inactive"}
                    </button>
                    <button
                      onClick={() => handleDelete(p.id)}
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
