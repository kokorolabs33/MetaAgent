"use client";

import type { DataArtifact } from "@/lib/types";

interface DataCardProps {
  artifact: DataArtifact;
}

function formatValue(value: unknown): string {
  if (value === null || value === undefined) return "\u2014";
  if (typeof value === "object") return JSON.stringify(value, null, 2);
  return String(value);
}

export function DataCard({ artifact }: DataCardProps) {
  const entries = Object.entries(artifact.data);

  return (
    <div className="mt-2 overflow-hidden rounded-lg border border-border">
      {artifact.title && (
        <div className="border-b border-border bg-gray-900/80 px-3 py-1.5">
          <span className="text-xs font-medium text-muted-foreground">
            {artifact.title}
          </span>
        </div>
      )}
      <div className="divide-y divide-gray-800">
        {entries.map(([key, value]) => (
          <div key={key} className="flex items-start gap-3 px-3 py-2">
            <span className="min-w-[100px] shrink-0 text-xs font-medium text-muted-foreground">
              {key.replace(/_/g, " ")}
            </span>
            <span className="break-all text-xs text-gray-300">
              {typeof value === "object" ? (
                <pre className="whitespace-pre-wrap text-xs">
                  {formatValue(value)}
                </pre>
              ) : (
                formatValue(value)
              )}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
