import type {
  Agent,
  AgentHealthOverview,
  AgentHealthDetail,
  AgentTaskDetail,
  Conversation,
  ConversationListItem,
  DashboardData,
  DiscoveredAgent,
  PaginatedAuditLogs,
  Task,
  TaskWithSubtasks,
  SubTask,
  Message,
  TimelineEvent,
  User,
  WorkflowTemplate,
  Policy,
  WebhookConfig,
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
    login: (email: string, name?: string) =>
      post<{ status: string; user_id: string }>("/api/auth/login", { email, name }),
    me: () => get<User>("/api/auth/me"),
    logout: () => post<void>("/api/auth/logout", {}),
  },
  agents: {
    list: (q?: string) => {
      const qs = q ? `?q=${encodeURIComponent(q)}` : "";
      return get<Agent[]>(`/api/agents${qs}`);
    },
    get: (id: string) => get<Agent>(`/api/agents/${id}`),
    create: (data: Partial<Agent>) => post<Agent>("/api/agents", data),
    update: (id: string, data: Partial<Agent>) => put<Agent>(`/api/agents/${id}`, data),
    delete: (id: string) => del(`/api/agents/${id}`),
    healthcheck: (id: string) => post<{ status: number; latency_ms: number }>(`/api/agents/${id}/healthcheck`, {}),
    testEndpoint: (endpoint: string) => post<{ status: number; latency_ms: number }>("/api/agents/test-endpoint", { endpoint }),
    discover: (url: string) => post<DiscoveredAgent>("/api/agents/discover", { url }),
    healthOverview: () => get<AgentHealthOverview[]>("/api/agents/health/overview"),
    health: (id: string) => get<AgentHealthDetail>(`/api/agents/${id}/health`),
  },
  tasks: {
    list: (params?: { status?: string; q?: string; page?: number; per_page?: number }) => {
      const searchParams = new URLSearchParams();
      if (params?.status) searchParams.set("status", params.status);
      if (params?.q) searchParams.set("q", params.q);
      if (params?.page) searchParams.set("page", params.page.toString());
      if (params?.per_page) searchParams.set("per_page", params.per_page.toString());
      const qs = searchParams.toString();
      return get<PaginatedTasks>(`/api/tasks${qs ? `?${qs}` : ""}`);
    },
    get: (id: string) => get<TaskWithSubtasks>(`/api/tasks/${id}`),
    create: (data: { title: string; description: string; template_id?: string }) => post<Task>("/api/tasks", data),
    cancel: (id: string) => post<void>(`/api/tasks/${id}/cancel`, {}),
    cost: (id: string) => get<{ total_cost_usd: number; total_input_tokens: number; total_output_tokens: number }>(`/api/tasks/${id}/cost`),
    subtasks: (id: string) => get<SubTask[]>(`/api/tasks/${id}/subtasks`),
    approve: (id: string, action: "approve" | "reject") =>
      post<{ status: string }>(`/api/tasks/${id}/approve`, { action }),
    timeline: (id: string) => get<TimelineEvent[]>(`/api/tasks/${id}/timeline`),
  },
  messages: {
    list: (taskId: string) => get<Message[]>(`/api/tasks/${taskId}/messages`),
    send: (taskId: string, content: string) =>
      post<Message | SendMessageResponse>(`/api/tasks/${taskId}/messages`, { content }),
  },
  templates: {
    list: () => get<WorkflowTemplate[]>("/api/templates"),
    get: (id: string) => get<WorkflowTemplate>(`/api/templates/${id}`),
    create: (data: { name: string; description: string; steps: Record<string, unknown>[]; variables?: Record<string, unknown>[] }) =>
      post<WorkflowTemplate>("/api/templates", data),
    update: (id: string, data: Partial<WorkflowTemplate>) =>
      put<WorkflowTemplate>(`/api/templates/${id}`, data),
    delete: (id: string) => del(`/api/templates/${id}`),
    createFromTask: (taskId: string, data: { name: string; description?: string }) =>
      post<WorkflowTemplate>(`/api/templates/from-task/${taskId}`, data),
    rollback: (id: string, version: number) =>
      post<WorkflowTemplate>(`/api/templates/${id}/rollback/${version}`, {}),
    analyze: (id: string) =>
      post<TemplateAnalysis>(`/api/templates/${id}/analyze`, {}),
  },
  policies: {
    list: () => get<Policy[]>("/api/policies"),
    create: (data: { name: string; rules: Record<string, unknown>; priority?: number }) =>
      post<Policy>("/api/policies", data),
    update: (id: string, data: Partial<Policy>) =>
      put<Policy>(`/api/policies/${id}`, data),
    delete: (id: string) => del(`/api/policies/${id}`),
  },
  analytics: {
    dashboard: (params?: { range?: string; status?: string }) => {
      const searchParams = new URLSearchParams();
      if (params?.range) searchParams.set("range", params.range);
      if (params?.status) searchParams.set("status", params.status);
      const qs = searchParams.toString();
      return get<DashboardData>(`/api/analytics/dashboard${qs ? `?${qs}` : ""}`);
    },
    agentTasks: (agentId: string, params?: { range?: string; status?: string }) => {
      const searchParams = new URLSearchParams();
      if (params?.range) searchParams.set("range", params.range);
      if (params?.status) searchParams.set("status", params.status);
      const qs = searchParams.toString();
      return get<AgentTaskDetail[]>(`/api/analytics/agents/${agentId}/tasks${qs ? `?${qs}` : ""}`);
    },
  },
  auditLogs: {
    list: (params?: { task_id?: string; agent_id?: string; type?: string; page?: number; per_page?: number }) => {
      const searchParams = new URLSearchParams();
      if (params?.task_id) searchParams.set("task_id", params.task_id);
      if (params?.agent_id) searchParams.set("agent_id", params.agent_id);
      if (params?.type) searchParams.set("type", params.type);
      if (params?.page) searchParams.set("page", params.page.toString());
      if (params?.per_page) searchParams.set("per_page", params.per_page.toString());
      const qs = searchParams.toString();
      return get<PaginatedAuditLogs>(`/api/audit-logs${qs ? `?${qs}` : ""}`);
    },
  },
  webhooks: {
    list: () => get<WebhookConfig[]>("/api/webhooks"),
    create: (data: { name: string; url: string; events: string[]; secret?: string }) =>
      post<WebhookConfig>("/api/webhooks", data),
    update: (id: string, data: Partial<WebhookConfig>) =>
      put<WebhookConfig>(`/api/webhooks/${id}`, data),
    delete: (id: string) => del(`/api/webhooks/${id}`),
    test: (id: string) =>
      post<{ success: boolean; status_code?: number; error?: string }>(`/api/webhooks/${id}/test`, {}),
  },
  conversations: {
    list: () => get<ConversationListItem[]>("/api/conversations"),
    create: (data?: { title?: string }) => post<Conversation>("/api/conversations", data ?? {}),
    get: (id: string) => get<Conversation>(`/api/conversations/${id}`),
    update: (id: string, data: { title: string }) => put<Conversation>(`/api/conversations/${id}`, data),
    delete: (id: string) => del(`/api/conversations/${id}`),
    messages: (id: string) => get<Message[]>(`/api/conversations/${id}/messages`),
    sendMessage: (id: string, content: string) => post<Message>(`/api/conversations/${id}/messages`, { content }),
    tasks: (id: string) => get<Task[]>(`/api/conversations/${id}/tasks`),
  },
  a2aConfig: {
    get: () => get<A2AConfig>("/api/a2a-config"),
    update: (data: { enabled?: boolean; name_override?: string; description_override?: string }) =>
      put<A2AConfig>("/api/a2a-config", data),
    refreshCard: () => post<Record<string, unknown>>("/api/a2a-config/refresh-card", {}),
  },
};

export interface TemplateAnalysis {
  template_id: string;
  execution_count: number;
  success_rate: number;
  avg_duration_seconds: number;
  avg_hitl_interventions: number;
  proposals: EvolutionProposal[];
}

export interface EvolutionProposal {
  type: string;
  description: string;
  reason: string;
}

export interface A2AConfig {
  enabled: boolean;
  name_override: string | null;
  description_override: string | null;
  aggregated_card: Record<string, unknown>;
}

export interface PaginatedTasks {
  items: Task[];
  total: number;
  page: number;
  per_page: number;
  pages: number;
}

export interface SendMessageResponse {
  message: Message;
  advisory_errors: string[];
}

export function isSendMessageResponse(
  result: Message | SendMessageResponse
): result is SendMessageResponse {
  return "advisory_errors" in result && "message" in result;
}
