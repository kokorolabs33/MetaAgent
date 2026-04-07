"use client";

import { useMemo } from "react";
import ReactMarkdown from "react-markdown";
import rehypeHighlight from "rehype-highlight";
import type { CodeArtifact } from "@/lib/types";

interface CodeBlockProps {
  artifact: CodeArtifact;
}

export function CodeBlock({ artifact }: CodeBlockProps) {
  const { language, code, filename } = artifact.data;

  // Wrap code in markdown fenced code block for rehype-highlight processing
  const markdown = useMemo(
    () => `\`\`\`${language}\n${code}\n\`\`\``,
    [language, code],
  );

  return (
    <div className="mt-2 overflow-hidden rounded-lg border border-border">
      {/* Header bar with language label and filename */}
      <div className="flex items-center justify-between border-b border-border bg-gray-900/80 px-3 py-1.5">
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
            {language}
          </span>
          {filename && (
            <span className="text-[10px] text-muted-foreground/60">
              {filename}
            </span>
          )}
        </div>
        {artifact.title && (
          <span className="text-xs text-muted-foreground">
            {artifact.title}
          </span>
        )}
      </div>
      {/* Code content with syntax highlighting */}
      <div className="[&_pre]:!mt-0 [&_pre]:!rounded-none [&_pre]:!border-0">
        <ReactMarkdown rehypePlugins={[rehypeHighlight]}>
          {markdown}
        </ReactMarkdown>
      </div>
    </div>
  );
}
