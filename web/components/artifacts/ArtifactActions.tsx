"use client";

import { useState, useCallback } from "react";
import { Copy, Download, Check } from "lucide-react";

interface ArtifactActionsProps {
  content: string;
  filename: string;
}

export function ArtifactActions({ content, filename }: ArtifactActionsProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(content);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback for non-secure contexts
      const textarea = document.createElement("textarea");
      textarea.value = content;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      document.body.removeChild(textarea);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }, [content]);

  const handleDownload = useCallback(() => {
    const blob = new Blob([content], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }, [content, filename]);

  return (
    <div className="absolute right-2 top-2 flex items-center gap-1 rounded-md border border-border bg-card/90 px-1 py-0.5 opacity-0 shadow-sm backdrop-blur-sm transition-opacity group-hover/artifact:opacity-100">
      <button
        type="button"
        onClick={() => void handleCopy()}
        className="flex items-center gap-1 rounded px-1.5 py-1 text-[10px] text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
        title="Copy to clipboard"
      >
        {copied ? (
          <Check className="size-3 text-green-400" />
        ) : (
          <Copy className="size-3" />
        )}
        <span>{copied ? "Copied" : "Copy"}</span>
      </button>
      <button
        type="button"
        onClick={handleDownload}
        className="flex items-center gap-1 rounded px-1.5 py-1 text-[10px] text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
        title={`Download as ${filename}`}
      >
        <Download className="size-3" />
        <span>Download</span>
      </button>
    </div>
  );
}
