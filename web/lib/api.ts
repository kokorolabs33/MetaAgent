import type {
  Organization,
  OrgListItem,
  OrgMemberWithUser,
  Agent,
  Task,
  TaskWithSubtasks,
  SubTask,
  Message,
} from "./types";

const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`, { credentials: "include" });
  if (!res.ok) throw new Error(`GET ${path} -> ${res.status}`);
  return res.json() as Promise<T>;
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`POST ${path} -> ${res.status}`);
  return res.json() as Promise<T>;
}

async function put<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`PUT ${path} -> ${res.status}`);
  return res.json() as Promise<T>;
}

async function del(path: string): Promise<void> {
  const res = await fetch(`${BASE}${path}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok) throw new Error(`DELETE ${path} -> ${res.status}`);
}

export const api = {
  auth: {
    logout: () => post<void>("/api/auth/logout", {}),
  },
  orgs: {
    list: () => get<OrgListItem[]>("/api/orgs"),
    create: (data: { name: string; slug: string }) =>
      post<Organization>("/api/orgs", data),
    get: (orgId: string) => get<Organization>(`/api/orgs/${orgId}`),
    update: (orgId: string, data: Partial<Organization>) =>
      put<Organization>(`/api/orgs/${orgId}`, data),
  },
  members: {
    list: (orgId: string) =>
      get<OrgMemberWithUser[]>(`/api/orgs/${orgId}/members`),
    invite: (orgId: string, data: { email: string; role: string }) =>
      post<{ status: string }>(`/api/orgs/${orgId}/members`, data),
    updateRole: (orgId: string, uid: string, data: { role: string }) =>
      put<{ status: string }>(`/api/orgs/${orgId}/members/${uid}`, data),
    remove: (orgId: string, uid: string) =>
      del(`/api/orgs/${orgId}/members/${uid}`),
  },
  agents: {
    list: (orgId: string) =>
      get<Agent[]>(`/api/orgs/${orgId}/agents`),
    get: (orgId: string, id: string) =>
      get<Agent>(`/api/orgs/${orgId}/agents/${id}`),
    create: (orgId: string, data: Partial<Agent>) =>
      post<Agent>(`/api/orgs/${orgId}/agents`, data),
    update: (orgId: string, id: string, data: Partial<Agent>) =>
      put<Agent>(`/api/orgs/${orgId}/agents/${id}`, data),
    delete: (orgId: string, id: string) =>
      del(`/api/orgs/${orgId}/agents/${id}`),
    healthcheck: (orgId: string, id: string) =>
      post<{ status: number; latency_ms: number }>(
        `/api/orgs/${orgId}/agents/${id}/healthcheck`,
        {},
      ),
  },
  tasks: {
    list: (orgId: string, status?: string) => {
      const params = status ? `?status=${status}` : "";
      return get<Task[]>(`/api/orgs/${orgId}/tasks${params}`);
    },
    get: (orgId: string, id: string) =>
      get<TaskWithSubtasks>(`/api/orgs/${orgId}/tasks/${id}`),
    create: (orgId: string, data: { title: string; description: string }) =>
      post<Task>(`/api/orgs/${orgId}/tasks`, data),
    cancel: (orgId: string, id: string) =>
      post<void>(`/api/orgs/${orgId}/tasks/${id}/cancel`, {}),
    cost: (orgId: string, id: string) =>
      get<{
        total_cost_usd: number;
        total_input_tokens: number;
        total_output_tokens: number;
      }>(`/api/orgs/${orgId}/tasks/${id}/cost`),
    subtasks: (orgId: string, id: string) =>
      get<SubTask[]>(`/api/orgs/${orgId}/tasks/${id}/subtasks`),
  },
  messages: {
    list: (orgId: string, taskId: string) =>
      get<Message[]>(`/api/orgs/${orgId}/tasks/${taskId}/messages`),
    send: (orgId: string, taskId: string, content: string) =>
      post<Message>(`/api/orgs/${orgId}/tasks/${taskId}/messages`, {
        content,
      }),
  },
};
