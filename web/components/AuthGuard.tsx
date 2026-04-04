"use client";

import { useEffect, useState, useMemo } from "react";
import { usePathname, useRouter } from "next/navigation";
import type { User } from "@/lib/types";
import { useAuthStore } from "@/lib/authStore";

const PUBLIC_PATHS = ["/login"];

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const isPublic = useMemo(() => PUBLIC_PATHS.includes(pathname), [pathname]);
  const [checked, setChecked] = useState(isPublic);
  const router = useRouter();
  const setUser = useAuthStore((s) => s.setUser);

  useEffect(() => {
    if (isPublic) return;

    const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
    fetch(`${BASE}/api/auth/me`, { credentials: "include" })
      .then(async (res) => {
        if (res.ok) {
          const user: User = await res.json();
          setUser(user);
          setChecked(true);
        } else {
          router.push("/login");
        }
      })
      .catch(() => {
        // If the API is unreachable (e.g. local mode), just render
        setChecked(true);
      });
  }, [isPublic, router, setUser]);

  if (!checked) {
    return (
      <div className="flex h-screen items-center justify-center bg-gray-950 text-gray-400">
        Loading...
      </div>
    );
  }

  return <>{children}</>;
}
