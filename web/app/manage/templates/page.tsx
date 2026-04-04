"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import type { WorkflowTemplate } from "@/lib/types";

export default function ManageTemplatesPage() {
  const [templates, setTemplates] = useState<WorkflowTemplate[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.templates.list().then(setTemplates).finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <p className="text-sm text-muted-foreground">Loading templates...</p>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b border-border px-6 py-4">
        <h1 className="text-lg font-semibold text-foreground">Workflow Templates</h1>
      </div>

      <div className="flex-1 overflow-auto p-6">
        {templates.length === 0 ? (
          <div className="flex h-64 items-center justify-center">
            <p className="text-sm text-muted-foreground">
              No templates yet. Create one from a completed task or manually.
            </p>
          </div>
        ) : (
          <div className="grid gap-3">
            {templates.map((t) => (
              <Link
                key={t.id}
                href={`/manage/templates/${t.id}`}
                className="block rounded-lg border border-border bg-card p-4 transition-colors hover:border-ring"
              >
                <div className="flex items-start justify-between">
                  <div>
                    <h2 className="font-semibold text-card-foreground">{t.name}</h2>
                    <p className="mt-1 text-sm text-muted-foreground">
                      {t.description || "No description"}
                    </p>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="text-xs text-muted-foreground">v{t.version}</span>
                    <span
                      className={`rounded px-2 py-0.5 text-xs ${
                        t.is_active
                          ? "bg-green-900/50 text-green-400"
                          : "bg-secondary text-muted-foreground"
                      }`}
                    >
                      {t.is_active ? "Active" : "Inactive"}
                    </span>
                  </div>
                </div>
              </Link>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
