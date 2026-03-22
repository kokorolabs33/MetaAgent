"use client";

import { useEffect } from "react";
import { useTaskHubStore } from "@/lib/store";

export default function Home() {
  const { loadOrgs, orgs, isLoading } = useTaskHubStore();

  useEffect(() => {
    loadOrgs();
  }, [loadOrgs]);

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-gray-950 text-white">
      <div className="border-b border-gray-800 bg-gray-900/80 backdrop-blur px-6 py-4">
        <div className="flex items-center gap-4 max-w-screen-xl mx-auto">
          <span className="font-semibold text-white text-sm">TaskHub V2</span>
        </div>
      </div>

      <div className="flex-1 flex items-center justify-center">
        {isLoading ? (
          <p className="text-gray-500 text-sm">Loading...</p>
        ) : orgs.length === 0 ? (
          <p className="text-gray-500 text-sm">
            No organisations found. Sign in and create one to get started.
          </p>
        ) : (
          <div className="space-y-2">
            {orgs.map((org) => (
              <div
                key={org.id}
                className="px-4 py-3 bg-gray-900 border border-gray-800 rounded-lg"
              >
                <p className="text-sm font-medium">{org.name}</p>
                <p className="text-xs text-gray-500">{org.slug}</p>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
