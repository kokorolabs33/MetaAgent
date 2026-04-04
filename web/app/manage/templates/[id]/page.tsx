"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft, Pencil, Save, X } from "lucide-react";
import { api } from "@/lib/api";
import type { TemplateAnalysis } from "@/lib/api";
import type { WorkflowTemplate } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { StepEditor, type TemplateStep } from "@/components/template/StepEditor";

function parseSteps(raw: Record<string, unknown>[]): TemplateStep[] {
  return (raw ?? []).map((s) => ({
    id: (s.id as string) ?? "",
    instruction_template: (s.instruction_template as string) ?? "",
    requires: s.requires as TemplateStep["requires"],
    depends_on: (s.depends_on as string[]) ?? [],
    mandatory: (s.mandatory as boolean) ?? false,
  }));
}

export default function ManageTemplateDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [template, setTemplate] = useState<WorkflowTemplate | null>(null);
  const [analysis, setAnalysis] = useState<TemplateAnalysis | null>(null);
  const [analyzing, setAnalyzing] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editSteps, setEditSteps] = useState<TemplateStep[]>([]);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (id) api.templates.get(id).then(setTemplate);
  }, [id]);

  const handleAnalyze = async () => {
    if (!id) return;
    setAnalyzing(true);
    try {
      const result = await api.templates.analyze(id);
      setAnalysis(result);
    } finally {
      setAnalyzing(false);
    }
  };

  const startEditing = () => {
    if (!template) return;
    setEditSteps(parseSteps(template.steps ?? []));
    setEditing(true);
  };

  const cancelEditing = () => {
    setEditing(false);
    setEditSteps([]);
  };

  const saveSteps = async () => {
    if (!id) return;
    setSaving(true);
    try {
      const updated = await api.templates.update(id, {
        steps: editSteps as unknown as Record<string, unknown>[],
      });
      setTemplate(updated);
      setEditing(false);
    } finally {
      setSaving(false);
    }
  };

  if (!template) {
    return (
      <div className="flex h-64 items-center justify-center">
        <p className="text-sm text-muted-foreground">Loading...</p>
      </div>
    );
  }

  const viewSteps = parseSteps(template.steps ?? []);

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center gap-3 border-b border-border px-6 py-4">
        <Link
          href="/manage/templates"
          className="rounded-md p-1 text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="size-4" />
        </Link>
        <h1 className="text-lg font-semibold text-foreground">{template.name}</h1>
      </div>

      <div className="flex-1 overflow-auto p-6">
        <p className="mb-6 text-sm text-muted-foreground">
          {template.description || "No description"}
        </p>

        <div className="mb-6 grid grid-cols-3 gap-4">
          <div className="rounded-lg border border-border bg-card p-3">
            <div className="text-xs text-muted-foreground">Version</div>
            <div className="text-lg font-semibold text-card-foreground">v{template.version}</div>
          </div>
          <div className="rounded-lg border border-border bg-card p-3">
            <div className="text-xs text-muted-foreground">Status</div>
            <div className={`text-lg font-semibold ${template.is_active ? "text-green-400" : "text-muted-foreground"}`}>
              {template.is_active ? "Active" : "Inactive"}
            </div>
          </div>
          <div className="rounded-lg border border-border bg-card p-3">
            <div className="text-xs text-muted-foreground">Source</div>
            <div className="text-sm text-card-foreground">
              {template.source_task_id ? "From task" : "Manual"}
            </div>
          </div>
        </div>

        <div className="mb-6">
          <div className="mb-2 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-foreground">Steps</h2>
            {!editing ? (
              <Button variant="outline" size="sm" onClick={startEditing}>
                <Pencil className="mr-1 size-3" /> Edit
              </Button>
            ) : (
              <div className="flex gap-2">
                <Button variant="outline" size="sm" onClick={cancelEditing}>
                  <X className="mr-1 size-3" /> Cancel
                </Button>
                <Button size="sm" onClick={saveSteps} disabled={saving}>
                  <Save className="mr-1 size-3" /> {saving ? "Saving..." : "Save"}
                </Button>
              </div>
            )}
          </div>

          {editing ? (
            <StepEditor steps={editSteps} onChange={setEditSteps} />
          ) : (
            <StepEditor steps={viewSteps} onChange={() => {}} readOnly />
          )}
        </div>

        {(template.variables ?? []).length > 0 && (
          <div className="mb-6">
            <h2 className="mb-2 text-sm font-semibold text-foreground">Variables</h2>
            <pre className="max-h-48 overflow-auto rounded-lg border border-border bg-card p-4 text-xs text-muted-foreground">
              {JSON.stringify(template.variables, null, 2)}
            </pre>
          </div>
        )}

        <div className="mb-6">
          <Button onClick={handleAnalyze} disabled={analyzing}>
            {analyzing ? "Analyzing..." : "Analyze & Get Evolution Proposals"}
          </Button>

          {analysis && (
            <div className="mt-4 rounded-lg border border-border bg-card p-4">
              <div className="mb-4 grid grid-cols-3 gap-4">
                <div>
                  <div className="text-xs text-muted-foreground">Executions</div>
                  <div className="text-lg font-semibold text-card-foreground">
                    {analysis.execution_count}
                  </div>
                </div>
                <div>
                  <div className="text-xs text-muted-foreground">Success Rate</div>
                  <div className="text-lg font-semibold text-card-foreground">
                    {(analysis.success_rate * 100).toFixed(0)}%
                  </div>
                </div>
                <div>
                  <div className="text-xs text-muted-foreground">Avg HITL</div>
                  <div className="text-lg font-semibold text-card-foreground">
                    {analysis.avg_hitl_interventions?.toFixed(1) ?? "0"}
                  </div>
                </div>
              </div>

              {(analysis.proposals ?? []).length > 0 ? (
                <div>
                  <h3 className="mb-2 text-sm font-semibold text-muted-foreground">
                    Evolution Proposals
                  </h3>
                  <div className="space-y-2">
                    {(analysis.proposals ?? []).map((p, i) => (
                      <div key={i} className="rounded border border-border bg-secondary p-3">
                        <div className="mb-1 flex items-center gap-2">
                          <span className="rounded bg-blue-900/50 px-1.5 py-0.5 text-xs text-blue-400">
                            {p.type}
                          </span>
                          <span className="text-sm text-card-foreground">{p.description}</span>
                        </div>
                        <p className="text-xs text-muted-foreground">{p.reason}</p>
                      </div>
                    ))}
                  </div>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">
                  No proposals -- template performing well or insufficient data.
                </p>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
