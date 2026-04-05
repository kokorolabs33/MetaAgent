"use client";

import { Suspense } from "react";
import { Loader2 } from "lucide-react";
import { TaskDashboard } from "@/components/dashboard/TaskDashboard";

export default function HomePage() {
  return (
    <Suspense
      fallback={
        <div className="flex h-full items-center justify-center">
          <Loader2
            className="size-6 animate-spin text-muted-foreground"
            aria-label="Loading dashboard"
          />
        </div>
      }
    >
      <TaskDashboard />
    </Suspense>
  );
}
