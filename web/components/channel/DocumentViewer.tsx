"use client";

import ReactMarkdown from "react-markdown";
import { useTaskHubStore } from "@/lib/store";
import { ScrollArea } from "@/components/ui/scroll-area";
import { FileText } from "lucide-react";

export function DocumentViewer() {
  const { currentChannel } = useTaskHubStore();

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center gap-2 px-4 py-2.5 border-b border-gray-800 bg-gray-900/50">
        <FileText className="w-4 h-4 text-gray-400" />
        <span className="text-xs font-medium text-gray-400 uppercase tracking-wider">
          Shared Context
        </span>
      </div>
      <ScrollArea className="flex-1 px-4 py-3">
        {currentChannel?.document ? (
          <div className="prose prose-invert prose-sm max-w-none text-gray-300">
            <ReactMarkdown>{currentChannel.document}</ReactMarkdown>
          </div>
        ) : (
          <div className="flex items-center justify-center h-32 text-gray-600 text-sm">
            Awaiting task assignment...
          </div>
        )}
      </ScrollArea>
    </div>
  );
}
