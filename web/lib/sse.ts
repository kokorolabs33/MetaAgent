// SSE connection manager — V2 placeholder
// Will be updated when event streaming is implemented

export interface SSEEvent<T = unknown> {
  type: string;
  data: T;
}

type SSEHandler = (event: SSEEvent) => void;

export function connectSSE(url: string, onEvent: SSEHandler): () => void {
  const source = new EventSource(url);

  source.onmessage = (e) => {
    try {
      const event: SSEEvent = JSON.parse(e.data as string);
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
