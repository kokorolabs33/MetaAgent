"use client";

/**
 * OrgProvider is kept as a passthrough for layout compatibility.
 * The org concept has been removed; this component is a no-op wrapper.
 */
export function OrgProvider({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}
