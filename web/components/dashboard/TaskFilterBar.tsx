"use client";

import { useCallback, useEffect, useState } from "react";
import { useSearchParams, usePathname, useRouter } from "next/navigation";
import { Search } from "lucide-react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { NewTaskDialog } from "@/components/dashboard/NewTaskDialog";

type TabKey = "all" | "running" | "completed" | "failed";

const TABS: { key: TabKey; label: string; statusParam: string | null }[] = [
  { key: "all", label: "All", statusParam: null },
  { key: "running", label: "Running", statusParam: "running" },
  { key: "completed", label: "Completed", statusParam: "completed" },
  { key: "failed", label: "Failed", statusParam: "failed" },
];

interface TaskFilterBarProps {
  /** Optional tab counts. When provided, renders "(N)" after each tab label. */
  counts?: Partial<Record<TabKey, number>>;
}

export function TaskFilterBar({ counts }: TaskFilterBarProps) {
  const searchParams = useSearchParams();
  const pathname = usePathname();
  const router = useRouter();

  const currentStatus = searchParams.get("status") ?? "";
  const currentQuery = searchParams.get("q") ?? "";
  const activeTab: TabKey =
    currentStatus === "running"
      ? "running"
      : currentStatus === "completed"
        ? "completed"
        : currentStatus === "failed"
          ? "failed"
          : "all";

  // Local state for the uncontrolled search input (debounced into URL).
  const [searchInput, setSearchInput] = useState(currentQuery);

  // Keep local state in sync if URL changes from elsewhere (back/forward).
  useEffect(() => {
    setSearchInput(currentQuery);
  }, [currentQuery]);

  const updateParams = useCallback(
    (updates: Record<string, string | null>) => {
      const params = new URLSearchParams(searchParams.toString());
      for (const [k, v] of Object.entries(updates)) {
        if (v === null || v === "") {
          params.delete(k);
        } else {
          params.set(k, v);
        }
      }
      // Reset page when any non-page filter changes.
      if (!("page" in updates)) {
        params.delete("page");
      }
      const qs = params.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname);
    },
    [searchParams, pathname, router],
  );

  const handleTabClick = useCallback(
    (tab: (typeof TABS)[number]) => {
      updateParams({ status: tab.statusParam });
    },
    [updateParams],
  );

  // Debounced search -> URL (300ms per UI-SPEC Interaction Contract).
  useEffect(() => {
    if (searchInput === currentQuery) return;
    const t = setTimeout(() => {
      updateParams({ q: searchInput || null });
    }, 300);
    return () => clearTimeout(t);
  }, [searchInput, currentQuery, updateParams]);

  return (
    <div
      className="flex items-center gap-1 border-b border-border px-6"
      role="tablist"
      aria-label="Task status filters"
    >
      {TABS.map((tab) => {
        const isActive = activeTab === tab.key;
        const count = counts?.[tab.key];
        return (
          <button
            key={tab.key}
            type="button"
            role="tab"
            aria-selected={isActive}
            onClick={() => handleTabClick(tab)}
            className={cn(
              "border-b-2 px-4 py-3 text-sm transition-colors -mb-px",
              isActive
                ? "border-primary font-bold text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground",
            )}
          >
            {tab.label}
            {count !== undefined && (
              <span className="ml-1 text-xs text-muted-foreground">
                ({count})
              </span>
            )}
          </button>
        );
      })}

      <div className="ml-auto flex items-center gap-2 py-2">
        <div className="relative">
          <Search
            className="pointer-events-none absolute left-2 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            type="search"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder="Search tasks"
            aria-label="Search tasks"
            className="w-48 pl-8"
          />
        </div>
        <NewTaskDialog />
      </div>
    </div>
  );
}
