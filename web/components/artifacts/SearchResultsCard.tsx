"use client";

import { ExternalLink } from "lucide-react";
import type { SearchResultsArtifact } from "@/lib/types";

interface SearchResultsCardProps {
  artifact: SearchResultsArtifact;
}

export function SearchResultsCard({ artifact }: SearchResultsCardProps) {
  return (
    <div className="mt-2 space-y-2">
      {artifact.title && (
        <p className="text-xs font-medium text-muted-foreground">
          {artifact.title}
        </p>
      )}
      {artifact.data.map((result, i) => (
        <a
          key={i}
          href={result.url}
          target="_blank"
          rel="noopener noreferrer"
          className="group block rounded-lg border border-border bg-card/50 p-3 transition-colors hover:border-blue-500/50 hover:bg-card"
        >
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium text-blue-400 group-hover:text-blue-300">
                {result.title}
              </p>
              <p className="mt-0.5 truncate text-xs text-green-400/70">
                {result.url}
              </p>
              <p className="mt-1 line-clamp-2 text-xs leading-relaxed text-muted-foreground">
                {result.snippet}
              </p>
            </div>
            <ExternalLink className="mt-0.5 size-3.5 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
          </div>
        </a>
      ))}
    </div>
  );
}
