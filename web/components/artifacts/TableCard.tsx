"use client";

import type { TableArtifact } from "@/lib/types";

interface TableCardProps {
  artifact: TableArtifact;
}

export function TableCard({ artifact }: TableCardProps) {
  const { headers, rows } = artifact.data;

  return (
    <div className="mt-2 overflow-hidden rounded-lg border border-border">
      {artifact.title && (
        <div className="border-b border-border bg-gray-900/80 px-3 py-1.5">
          <span className="text-xs font-medium text-muted-foreground">
            {artifact.title}
          </span>
        </div>
      )}
      <div className="overflow-auto">
        <table className="w-full border-collapse text-xs">
          <thead>
            <tr className="border-b border-gray-700 bg-gray-900/40">
              {headers.map((header, i) => (
                <th
                  key={i}
                  className="px-3 py-2 text-left font-semibold text-foreground"
                >
                  {header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row, ri) => (
              <tr
                key={ri}
                className="border-b border-gray-800 last:border-0"
              >
                {row.map((cell, ci) => (
                  <td key={ci} className="px-3 py-1.5 text-gray-300">
                    {cell}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
