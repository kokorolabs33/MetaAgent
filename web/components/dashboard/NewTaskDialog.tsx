"use client";

import { useState, useCallback, useEffect } from "react";
import { useRouter } from "next/navigation";
import { Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useTaskStore } from "@/lib/store";
import { useToast } from "@/components/ui/toast";
import { api } from "@/lib/api";
import type { WorkflowTemplate } from "@/lib/types";

export function NewTaskDialog() {
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [templateId, setTemplateId] = useState("");
  const [templates, setTemplates] = useState<WorkflowTemplate[]>([]);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const createTask = useTaskStore((s) => s.createTask);
  const router = useRouter();
  const { addToast } = useToast();

  useEffect(() => {
    if (open) {
      api.templates.list().then(setTemplates).catch(() => setTemplates([]));
    }
  }, [open]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!title.trim()) return;

      setIsSubmitting(true);
      setError(null);

      try {
        const task = await createTask(
          title.trim(),
          description.trim(),
          templateId || undefined,
        );
        setOpen(false);
        setTitle("");
        setDescription("");
        setTemplateId("");
        addToast("success", `Task "${title.trim()}" created`);
        router.push(`/tasks/${task.id}`);
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Failed to create task";
        setError(msg);
        addToast("error", msg);
      } finally {
        setIsSubmitting(false);
      }
    },
    [title, description, templateId, createTask, router, addToast],
  );

  return (
    <>
      <Button onClick={() => setOpen(true)}>
        <Plus className="size-4" />
        New Task
      </Button>

      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div
            className="absolute inset-0 bg-black/60"
            onClick={() => setOpen(false)}
          />
          <div className="relative w-full max-w-md rounded-xl border border-border bg-card p-6 shadow-xl">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold text-card-foreground">
                Create Task
              </h2>
              <button
                onClick={() => setOpen(false)}
                className="rounded-md p-1 text-muted-foreground transition-colors hover:text-foreground"
              >
                <X className="size-4" />
              </button>
            </div>

            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <label
                  htmlFor="task-title"
                  className="text-sm font-medium text-card-foreground"
                >
                  Title
                </label>
                <Input
                  id="task-title"
                  placeholder="What needs to be done?"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  required
                  autoFocus
                />
              </div>

              <div className="space-y-2">
                <label
                  htmlFor="task-description"
                  className="text-sm font-medium text-card-foreground"
                >
                  Description
                </label>
                <textarea
                  id="task-description"
                  placeholder="Describe the task in detail..."
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  rows={4}
                  className="w-full rounded-lg border border-input bg-transparent px-2.5 py-2 text-sm transition-colors placeholder:text-muted-foreground focus-visible:border-ring focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
                />
              </div>

              {templates.length > 0 && (
                <div className="space-y-2">
                  <label
                    htmlFor="task-template"
                    className="text-sm font-medium text-card-foreground"
                  >
                    Template (optional)
                  </label>
                  <select
                    id="task-template"
                    value={templateId}
                    onChange={(e) => setTemplateId(e.target.value)}
                    className="w-full rounded-lg border border-input bg-transparent px-2.5 py-2 text-sm transition-colors focus-visible:border-ring focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
                  >
                    <option value="" className="bg-card">
                      No template
                    </option>
                    {templates
                      .filter((t) => t.is_active)
                      .map((t) => (
                        <option key={t.id} value={t.id} className="bg-card">
                          {t.name} (v{t.version})
                        </option>
                      ))}
                  </select>
                </div>
              )}

              {error && (
                <p className="text-sm text-destructive">{error}</p>
              )}

              <div className="flex justify-end gap-2">
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => setOpen(false)}
                >
                  Cancel
                </Button>
                <Button type="submit" disabled={isSubmitting || !title.trim()}>
                  {isSubmitting ? "Creating..." : "Create Task"}
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  );
}
