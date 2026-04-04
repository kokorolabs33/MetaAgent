"use client";

import { useEffect, useCallback } from "react";
import Image from "next/image";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  Plus,
  Settings,
  LogOut,
  MessageSquare,
  Loader2,
  Trash2,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useConversationStore } from "@/lib/conversationStore";
import { useAuthStore } from "@/lib/authStore";
import type { ConversationListItem } from "@/lib/types";

function groupByDate(items: ConversationListItem[]) {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const yesterday = new Date(today);
  yesterday.setDate(yesterday.getDate() - 1);

  const groups: { label: string; items: ConversationListItem[] }[] = [];
  const todayItems = items.filter((i) => new Date(i.updated_at) >= today);
  const yesterdayItems = items.filter((i) => {
    const d = new Date(i.updated_at);
    return d >= yesterday && d < today;
  });
  const earlierItems = items.filter(
    (i) => new Date(i.updated_at) < yesterday,
  );

  if (todayItems.length) groups.push({ label: "Today", items: todayItems });
  if (yesterdayItems.length)
    groups.push({ label: "Yesterday", items: yesterdayItems });
  if (earlierItems.length)
    groups.push({ label: "Earlier", items: earlierItems });
  return groups;
}

function statusIndicator(status: string) {
  if (!status) return null;
  const colors: Record<string, string> = {
    running: "bg-amber-500",
    planning: "bg-blue-500",
    completed: "bg-green-500",
    failed: "bg-red-500",
    pending: "bg-gray-500",
    approval_required: "bg-amber-600",
  };
  return (
    <span
      className={cn("inline-block size-2 rounded-full", colors[status] ?? "bg-gray-500")}
      title={status}
    />
  );
}

export function ConversationSidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const clearUser = useAuthStore((s) => s.clearUser);

  const {
    conversations,
    isLoadingList,
    loadConversations,
    createConversation,
    deleteConversation,
  } = useConversationStore();

  useEffect(() => {
    void loadConversations();
  }, [loadConversations]);

  const handleNewChat = useCallback(async () => {
    try {
      const conv = await createConversation();
      router.push(`/c/${conv.id}`);
    } catch {
      // ignore
    }
  }, [createConversation, router]);

  const handleDelete = useCallback(
    async (e: React.MouseEvent, id: string) => {
      e.preventDefault();
      e.stopPropagation();
      await deleteConversation(id);
      if (pathname === `/c/${id}`) {
        router.push("/");
      }
    },
    [deleteConversation, pathname, router],
  );

  const handleLogout = async () => {
    try {
      const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
      await fetch(`${BASE}/api/auth/logout`, {
        method: "POST",
        credentials: "include",
      });
    } catch {
      // ignore network errors during logout
    }
    clearUser();
    router.push("/login");
  };

  const groups = groupByDate(conversations);

  // Extract active conversation id from path
  const activeId = pathname.startsWith("/c/")
    ? pathname.split("/")[2]
    : null;

  return (
    <aside className="flex h-screen w-64 flex-col border-r border-border bg-gray-950">
      {/* Logo */}
      <div className="flex items-center gap-2.5 px-5 py-5">
        <Image src="/logo.svg" alt="TaskHub" width={28} height={28} />
        <span className="text-sm font-semibold text-white">TaskHub</span>
      </div>

      {/* New Chat */}
      <div className="px-3 pb-2">
        <button
          onClick={() => void handleNewChat()}
          className="flex w-full items-center gap-2 rounded-lg border border-border px-3 py-2 text-sm font-medium text-gray-300 transition-colors hover:bg-secondary/50 hover:text-white"
        >
          <Plus className="size-4" />
          New Chat
        </button>
      </div>

      {/* Conversation list */}
      <nav className="flex-1 overflow-y-auto px-3">
        {isLoadingList && conversations.length === 0 ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="size-5 animate-spin text-muted-foreground" />
          </div>
        ) : groups.length === 0 ? (
          <div className="px-3 py-8 text-center text-xs text-muted-foreground">
            No conversations yet
          </div>
        ) : (
          groups.map((group) => (
            <div key={group.label} className="mb-3">
              <div className="mb-1 px-3 text-[10px] font-semibold uppercase tracking-wider text-gray-500">
                {group.label}
              </div>
              {group.items.map((conv) => {
                const isActive = activeId === conv.id;
                return (
                  <Link
                    key={conv.id}
                    href={`/c/${conv.id}`}
                    className={cn(
                      "group flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors",
                      isActive
                        ? "bg-secondary text-white"
                        : "text-gray-400 hover:bg-secondary/50 hover:text-gray-200",
                    )}
                  >
                    <MessageSquare className="size-4 shrink-0" />
                    <span className="flex-1 truncate">{conv.title}</span>
                    <div className="flex items-center gap-1.5 shrink-0">
                      {statusIndicator(conv.latest_status)}
                      {conv.task_count > 0 && (
                        <span className="text-[10px] text-gray-500">
                          {conv.task_count}
                        </span>
                      )}
                      <button
                        onClick={(e) => void handleDelete(e, conv.id)}
                        className="hidden rounded p-0.5 text-gray-500 transition-colors hover:text-red-400 group-hover:block"
                        title="Delete conversation"
                      >
                        <Trash2 className="size-3" />
                      </button>
                    </div>
                  </Link>
                );
              })}
            </div>
          ))
        )}
      </nav>

      {/* Bottom: Settings + User */}
      <div className="border-t border-border px-3 py-2">
        <Link
          href="/manage/agents"
          className={cn(
            "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
            pathname.startsWith("/manage")
              ? "bg-secondary text-white"
              : "text-gray-400 hover:bg-secondary/50 hover:text-gray-200",
          )}
        >
          <Settings className="size-4" />
          Settings
        </Link>
      </div>

      {user && (
        <div className="border-t border-border px-3 py-3">
          <div className="flex items-center gap-3">
            <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-blue-600 text-xs font-medium text-white">
              {user.name.charAt(0).toUpperCase()}
            </div>
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-white">
                {user.name}
              </p>
              <p className="truncate text-xs text-gray-400">{user.email}</p>
            </div>
            <button
              onClick={() => void handleLogout()}
              title="Sign out"
              className="rounded-md p-1.5 text-gray-400 transition-colors hover:bg-secondary hover:text-white"
            >
              <LogOut className="size-4" />
            </button>
          </div>
        </div>
      )}
    </aside>
  );
}
