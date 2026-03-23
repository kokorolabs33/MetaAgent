const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export type SSEEventHandler = (event: {
  id: string;
  type: string;
  data: Record<string, unknown>;
}) => void;

export function connectSSE(
  orgId: string,
  taskId: string,
  onEvent: SSEEventHandler,
  onError?: (error: Event) => void,
): () => void {
  const url = `${BASE}/api/orgs/${orgId}/tasks/${taskId}/events`;
  const eventSource = new EventSource(url, { withCredentials: true });

  eventSource.onmessage = (e: MessageEvent) => {
    try {
      const parsed = JSON.parse(e.data as string) as Record<string, unknown>;
      onEvent({
        id: e.lastEventId,
        type: parsed.type as string,
        data: (parsed.data as Record<string, unknown>) ?? parsed,
      });
    } catch {
      // ignore malformed events
    }
  };

  eventSource.onerror = (e: Event) => {
    if (onError) onError(e);
    // EventSource auto-reconnects with Last-Event-ID header
  };

  return () => {
    eventSource.close();
  };
}
