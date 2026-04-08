"use client";

import Image from "next/image";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  Bot,
  FileText,
  Shield,
  Server,
  BarChart3,
  HeartPulse,
  ScrollText,
  LogOut,
  Bell,
  ArrowLeft,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/lib/authStore";

const navItems = [
  { href: "/manage/agents", label: "Agents", icon: Bot },
  { href: "/manage/agents/health", label: "Agent Health", icon: HeartPulse },
  { href: "/manage/templates", label: "Templates", icon: FileText },
  { href: "/manage/analytics", label: "Analytics", icon: BarChart3 },
  { href: "/manage/audit", label: "Audit Log", icon: ScrollText },
];

const settingsItems = [
  { href: "/manage/settings/policies", label: "Policies", icon: Shield },
  { href: "/manage/settings/webhooks", label: "Webhooks", icon: Bell },
  { href: "/manage/settings/a2a-server", label: "A2A Server", icon: Server },
];

export default function ManageLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const clearUser = useAuthStore((s) => s.clearUser);

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

  return (
    <div className="flex h-screen overflow-hidden">
      <aside className="flex h-screen w-60 flex-col border-r border-border bg-gray-950">
        <div className="flex items-center gap-2.5 px-5 py-5">
          <Image src="/logo.svg" alt="MetaAgent" width={28} height={28} />
          <span className="text-sm font-semibold text-white">MetaAgent</span>
        </div>

        <nav className="flex flex-1 flex-col gap-1 px-3">
          {/* Back to chat */}
          <Link
            href="/"
            className="flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-gray-400 transition-colors hover:bg-secondary/50 hover:text-gray-200 mb-2"
          >
            <ArrowLeft className="size-4" />
            Back to Chat
          </Link>

          <div className="mb-1 px-3 text-xs font-medium uppercase tracking-wider text-gray-500">
            Management
          </div>

          {navItems.map((item) => {
            const isActive =
              item.href === "/manage/agents"
                ? pathname === "/manage/agents" ||
                  (pathname.startsWith("/manage/agents/") &&
                    !pathname.startsWith("/manage/agents/health"))
                : pathname.startsWith(item.href);

            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                  isActive
                    ? "bg-secondary text-white"
                    : "text-gray-400 hover:bg-secondary/50 hover:text-gray-200",
                )}
              >
                <item.icon className="size-4" />
                {item.label}
              </Link>
            );
          })}

          <div className="mt-4 mb-1 px-3 text-xs font-medium uppercase tracking-wider text-gray-500">
            Settings
          </div>
          {settingsItems.map((item) => {
            const isActive = pathname.startsWith(item.href);

            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                  isActive
                    ? "bg-secondary text-white"
                    : "text-gray-400 hover:bg-secondary/50 hover:text-gray-200",
                )}
              >
                <item.icon className="size-4" />
                {item.label}
              </Link>
            );
          })}
        </nav>

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
      <main className="flex-1 overflow-auto">{children}</main>
    </div>
  );
}
