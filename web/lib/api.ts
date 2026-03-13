import type { Agent, ChannelDetail, Task } from "./types";

const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`);
  if (!res.ok) throw new Error(`GET ${path} → ${res.status}`);
  return res.json();
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`POST ${path} → ${res.status}`);
  return res.json();
}

export const api = {
  agents: {
    list: () => get<Agent[]>("/api/agents"),
    create: (data: Partial<Agent>) => post<Agent>("/api/agents", data),
  },
  tasks: {
    list: () => get<Task[]>("/api/tasks"),
    get: (id: string) => get<Task>(`/api/tasks/${id}`),
    create: (data: { title?: string; description: string }) =>
      post<Task>("/api/tasks", data),
    getChannel: (id: string) =>
      get<{ channel_id: string }>(`/api/tasks/${id}/channel`),
  },
  channels: {
    get: (id: string) => get<ChannelDetail>(`/api/channels/${id}`),
    streamUrl: (id: string) => `${BASE}/api/channels/${id}/stream`,
  },
};
