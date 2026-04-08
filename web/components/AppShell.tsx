"use client";

import { usePathname } from "next/navigation";
import { ConversationSidebar } from "@/components/conversation/ConversationSidebar";
import { AuthGuard } from "@/components/AuthGuard";
import { OrgProvider } from "@/components/OrgProvider";

const NO_SHELL_PATHS = ["/login"];

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const isNoShell = NO_SHELL_PATHS.includes(pathname);
  const isManage = pathname.startsWith("/manage");
  const isChatRoute = pathname === "/" || pathname.startsWith("/c/");

  return (
    <AuthGuard>
      {isNoShell ? (
        <>{children}</>
      ) : isManage ? (
        // Management routes have their own layout with Nav sidebar
        <OrgProvider>{children}</OrgProvider>
      ) : isChatRoute ? (
        // Chat routes: conversation sidebar + content
        <div className="flex h-screen overflow-hidden">
          <ConversationSidebar />
          <main className="flex-1 overflow-hidden">
            <OrgProvider>{children}</OrgProvider>
          </main>
        </div>
      ) : (
        // Fallback for old routes (e.g. /agents, /tasks/[id])
        <div className="flex h-screen overflow-hidden">
          <ConversationSidebar />
          <main className="flex-1 overflow-auto">
            <OrgProvider>{children}</OrgProvider>
          </main>
        </div>
      )}
    </AuthGuard>
  );
}
