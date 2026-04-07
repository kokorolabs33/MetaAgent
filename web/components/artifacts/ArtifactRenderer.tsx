"use client";

import type { Artifact } from "@/lib/types";
import { SearchResultsCard } from "./SearchResultsCard";
import { CodeBlock } from "./CodeBlock";
import { TableCard } from "./TableCard";
import { DataCard } from "./DataCard";
import { ArtifactActions } from "./ArtifactActions";

interface ArtifactRendererProps {
  artifacts: Artifact[];
}

function getArtifactContent(artifact: Artifact): string {
  switch (artifact.type) {
    case "search_results":
      return artifact.data
        .map((r) => `${r.title}\n${r.url}\n${r.snippet}`)
        .join("\n\n");
    case "code":
      return artifact.data.code;
    case "table": {
      const { headers, rows } = artifact.data;
      return [headers.join("\t"), ...rows.map((r) => r.join("\t"))].join("\n");
    }
    case "data":
      return JSON.stringify(artifact.data, null, 2);
    default:
      return JSON.stringify(artifact, null, 2);
  }
}

function getDownloadFilename(artifact: Artifact): string {
  switch (artifact.type) {
    case "search_results":
      return "search-results.txt";
    case "code":
      return artifact.data.filename || `code.${artifact.data.language || "txt"}`;
    case "table":
      return "table.tsv";
    case "data":
      return "data.json";
    default:
      return "artifact.json";
  }
}

function renderSingleArtifact(artifact: Artifact): React.ReactNode {
  switch (artifact.type) {
    case "search_results":
      return <SearchResultsCard artifact={artifact} />;
    case "code":
      return <CodeBlock artifact={artifact} />;
    case "table":
      return <TableCard artifact={artifact} />;
    case "data":
      return <DataCard artifact={artifact} />;
    default:
      // Fallback: render unknown types as raw JSON (never crash)
      return (
        <pre className="mt-2 max-h-48 overflow-auto rounded-lg border border-border bg-gray-900 p-3 text-xs text-gray-300">
          {JSON.stringify(artifact, null, 2)}
        </pre>
      );
  }
}

export function ArtifactRenderer({ artifacts }: ArtifactRendererProps) {
  return (
    <div className="space-y-2">
      {artifacts.map((artifact, i) => (
        <div key={i} className="group/artifact relative">
          {renderSingleArtifact(artifact)}
          <ArtifactActions
            content={getArtifactContent(artifact)}
            filename={getDownloadFilename(artifact)}
          />
        </div>
      ))}
    </div>
  );
}
