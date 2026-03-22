"use client";

import { Zap } from "lucide-react";

export function TaskBar() {
  return (
    <div className="border-b border-gray-800 bg-gray-900/80 backdrop-blur px-6 py-4">
      <div className="flex items-center gap-4 max-w-screen-xl mx-auto">
        <div className="flex items-center gap-2 mr-4">
          <div className="w-7 h-7 rounded-lg bg-purple-600 flex items-center justify-center">
            <Zap className="w-4 h-4 text-white" />
          </div>
          <span className="font-semibold text-white text-sm">TaskHub</span>
        </div>
        <span className="text-sm text-gray-500">V2 — coming soon</span>
      </div>
    </div>
  );
}
