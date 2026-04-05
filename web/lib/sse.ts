const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export type SSEEventHandler = (event: {
  id: string;
  type: string;
  data: Record<string, unknown>;
  subtask_id?: string;
  task_id?: string;
  actor_type?: string;
  actor_id?: string;
  created_at?: string;
}) => void;

export function connectSSE(
  taskId: string,
  onEvent: SSEEventHandler,
  onError?: (error: Event) => void,
): () => void {
  const url = `${BASE}/api/tasks/${taskId}/events`;
  const eventSource = new EventSource(url, { withCredentials: true });

  eventSource.onmessage = (e: MessageEvent) => {
    try {
      const parsed = JSON.parse(e.data as string) as Record<string, unknown>;
      // Pass the full event object — the store needs subtask_id, actor_id, etc.
      // from the top level, not just the nested data field.
      onEvent({
        id: e.lastEventId,
        type: parsed.type as string,
        data: (parsed.data as Record<string, unknown>) ?? {},
        ...(parsed.subtask_id ? { subtask_id: parsed.subtask_id as string } : {}),
        ...(parsed.task_id ? { task_id: parsed.task_id as string } : {}),
        ...(parsed.actor_type ? { actor_type: parsed.actor_type as string } : {}),
        ...(parsed.actor_id ? { actor_id: parsed.actor_id as string } : {}),
        ...(parsed.created_at ? { created_at: parsed.created_at as string } : {}),
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

export function connectConversationSSE(
  conversationId: string,
  onEvent: SSEEventHandler,
  onError?: (error: Event) => void,
): () => void {
  const url = `${BASE}/api/conversations/${conversationId}/events`;
  const eventSource = new EventSource(url, { withCredentials: true });

  eventSource.onmessage = (e: MessageEvent) => {
    try {
      const parsed = JSON.parse(e.data as string) as Record<string, unknown>;
      onEvent({
        id: e.lastEventId,
        type: parsed.type as string,
        data: (parsed.data as Record<string, unknown>) ?? {},
        ...(parsed.subtask_id ? { subtask_id: parsed.subtask_id as string } : {}),
        ...(parsed.task_id ? { task_id: parsed.task_id as string } : {}),
        ...(parsed.actor_type ? { actor_type: parsed.actor_type as string } : {}),
        ...(parsed.actor_id ? { actor_id: parsed.actor_id as string } : {}),
        ...(parsed.created_at ? { created_at: parsed.created_at as string } : {}),
      });
    } catch {
      // ignore malformed events
    }
  };

  eventSource.onerror = (e: Event) => {
    if (onError) onError(e);
  };

  return () => {
    eventSource.close();
  };
}

export function connectAgentStatusSSE(
  onEvent: SSEEventHandler,
  onError?: (error: Event) => void,
): () => void {
  const url = `${BASE}/api/agents/stream`;
  const eventSource = new EventSource(url, { withCredentials: true });

  eventSource.onmessage = (e: MessageEvent) => {
    try {
      const parsed = JSON.parse(e.data as string) as Record<string, unknown>;
      onEvent({
        id: e.lastEventId,
        type: parsed.type as string,
        data: (parsed.data as Record<string, unknown>) ?? {},
        ...(parsed.actor_type ? { actor_type: parsed.actor_type as string } : {}),
        ...(parsed.actor_id ? { actor_id: parsed.actor_id as string } : {}),
        ...(parsed.created_at ? { created_at: parsed.created_at as string } : {}),
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
