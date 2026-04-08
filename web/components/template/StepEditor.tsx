"use client";

import { useState } from "react";
import { Plus, Trash2, GripVertical } from "lucide-react";
import { Button } from "@/components/ui/button";

interface TemplateStep {
  id: string;
  instruction_template: string;
  requires?: { skills?: string[]; tags?: string[] };
  depends_on?: string[];
  mandatory?: boolean;
}

interface StepEditorProps {
  steps: TemplateStep[];
  onChange: (steps: TemplateStep[]) => void;
  readOnly?: boolean;
}

export type { TemplateStep };

export function StepEditor({ steps, onChange, readOnly = false }: StepEditorProps) {
  const addStep = () => {
    const newId = `step_${steps.length + 1}`;
    onChange([...steps, {
      id: newId,
      instruction_template: "",
      requires: { skills: [], tags: [] },
      depends_on: [],
      mandatory: false,
    }]);
  };

  const updateStep = (index: number, updates: Partial<TemplateStep>) => {
    const updated = steps.map((s, i) => i === index ? { ...s, ...updates } : s);
    onChange(updated);
  };

  const removeStep = (index: number) => {
    const removedId = steps[index].id;
    // Also remove from other steps' depends_on
    const updated = steps
      .filter((_, i) => i !== index)
      .map(s => ({
        ...s,
        depends_on: s.depends_on?.filter(d => d !== removedId),
      }));
    onChange(updated);
  };

  const allStepIds = steps.map(s => s.id);

  return (
    <div className="space-y-3">
      {steps.map((step, i) => (
        <div key={i} className="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
          <div className="flex items-start gap-3">
            <GripVertical className="mt-1 size-4 shrink-0 text-zinc-600" />
            <div className="flex-1 space-y-3">
              {/* Step ID + mandatory toggle */}
              <div className="flex items-center gap-3">
                <input
                  value={step.id}
                  onChange={(e) => updateStep(i, { id: e.target.value })}
                  disabled={readOnly}
                  className="w-32 rounded border border-zinc-700 bg-zinc-800 px-2 py-1 font-mono text-sm text-zinc-100"
                  placeholder="step_id"
                />
                <label className="flex items-center gap-1.5 text-xs text-zinc-400">
                  <input
                    type="checkbox"
                    checked={step.mandatory ?? false}
                    onChange={(e) => updateStep(i, { mandatory: e.target.checked })}
                    disabled={readOnly}
                    className="rounded border-zinc-600"
                  />
                  Mandatory
                </label>
              </div>

              {/* Instruction */}
              <textarea
                value={step.instruction_template}
                onChange={(e) => updateStep(i, { instruction_template: e.target.value })}
                disabled={readOnly}
                rows={2}
                placeholder="Instruction for this step..."
                className="w-full resize-none rounded border border-zinc-700 bg-zinc-800 px-3 py-2 text-sm text-zinc-100"
              />

              {/* Required skills */}
              <div>
                <label className="mb-1 block text-xs text-zinc-500">Required Skills</label>
                <SkillTagInput
                  value={step.requires?.skills ?? []}
                  onChange={(skills) => updateStep(i, { requires: { ...step.requires, skills } })}
                  disabled={readOnly}
                />
              </div>

              {/* Dependencies */}
              <div>
                <label className="mb-1 block text-xs text-zinc-500">Depends On</label>
                <div className="flex flex-wrap gap-1.5">
                  {allStepIds.filter(id => id !== step.id).map(depId => (
                    <label key={depId} className="flex items-center gap-1 text-xs text-zinc-400">
                      <input
                        type="checkbox"
                        checked={step.depends_on?.includes(depId) ?? false}
                        onChange={(e) => {
                          const deps = step.depends_on ?? [];
                          const newDeps = e.target.checked
                            ? [...deps, depId]
                            : deps.filter(d => d !== depId);
                          updateStep(i, { depends_on: newDeps });
                        }}
                        disabled={readOnly}
                        className="rounded border-zinc-600"
                      />
                      {depId}
                    </label>
                  ))}
                </div>
              </div>
            </div>

            {!readOnly && (
              <button onClick={() => removeStep(i)} className="mt-1 text-red-500 hover:text-red-400">
                <Trash2 className="size-4" />
              </button>
            )}
          </div>
        </div>
      ))}

      {!readOnly && (
        <Button variant="outline" size="sm" onClick={addStep} className="w-full">
          <Plus className="mr-1 size-4" /> Add Step
        </Button>
      )}
    </div>
  );
}

// Simple tag input for skills
function SkillTagInput({ value, onChange, disabled }: { value: string[]; onChange: (v: string[]) => void; disabled?: boolean }) {
  const [input, setInput] = useState("");

  const addTag = () => {
    const tag = input.trim();
    if (tag && !value.includes(tag)) {
      onChange([...value, tag]);
      setInput("");
    }
  };

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {value.map((tag) => (
        <span key={tag} className="flex items-center gap-1 rounded bg-blue-900/30 px-2 py-0.5 text-xs text-blue-400">
          {tag}
          {!disabled && (
            <button onClick={() => onChange(value.filter(t => t !== tag))} className="hover:text-blue-300">&times;</button>
          )}
        </span>
      ))}
      {!disabled && (
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addTag(); } }}
          onBlur={addTag}
          placeholder="Add skill..."
          className="w-24 bg-transparent text-xs text-zinc-300 outline-none placeholder:text-zinc-600"
        />
      )}
    </div>
  );
}
