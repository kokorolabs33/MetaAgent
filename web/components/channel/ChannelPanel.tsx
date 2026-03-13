"use client";

import { DocumentViewer } from "./DocumentViewer";
import { MessageFeed } from "./MessageFeed";

export function ChannelPanel() {
  return (
    <div className="flex flex-col h-full">
      <div className="h-[45%] border-b border-gray-800 overflow-hidden">
        <DocumentViewer />
      </div>
      <div className="flex-1 overflow-hidden">
        <MessageFeed />
      </div>
    </div>
  );
}
