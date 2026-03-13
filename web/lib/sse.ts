import type { SSEEvent } from "./types";

type SSEHandler = (event: SSEEvent) => void;

export function connectSSE(url: string, onEvent: SSEHandler): () => void {
  const source = new EventSource(url);

  source.onmessage = (e) => {
    try {
      const event: SSEEvent = JSON.parse(e.data);
      onEvent(event);
    } catch {
      console.error("SSE parse error:", e.data);
    }
  };

  source.onerror = (e) => {
    console.error("SSE error:", e);
  };

  return () => source.close();
}
