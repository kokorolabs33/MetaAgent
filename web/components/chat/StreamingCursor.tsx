"use client";

/**
 * Blinking cursor shown at the end of streaming agent messages.
 * Subtle thin bar (not a heavy block cursor) per D-06.
 */
export function StreamingCursor() {
  return (
    <span
      className="ml-0.5 inline-block h-4 w-0.5 animate-pulse bg-blue-400"
      aria-hidden="true"
    />
  );
}
