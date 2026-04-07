// web/lib/types.ts — TypeScript interfaces matching Go models

export interface User {
  id: string;
  email: string;
  name: string;
  avatar_url: string;
  created_at: string;
}

export interface PageResponse<T> {
  items: T[];
  next_cursor: string | null;
  has_more: boolean;
}

// Agent Registry

export interface Agent {
  id: string;
  name: string;
  version: string;
  description: string;
  endpoint: string;
  agent_card_url: string;
  agent_card?: Record<string, unknown>;
  card_fetched_at?: string;
  capabilities: string[];
  skills?: AgentSkill[];
  status: "active" | "inactive" | "degraded";
  is_online: boolean;
  last_health_check?: string;
  created_at: string;
  updated_at: string;
}

export interface AgentSkill {
  id: string;
  name: string;
  description: string;
}

export interface DiscoveredAgent {
  name: string;
  description: string;
  version: string;
  url: string;
  skills: AgentSkill[];
  capabilities: string[];
  raw_card: Record<string, unknown>;
}

// Task System

export interface Task {
  id: string;
  title: string;
  description: string;
  status: "pending" | "planning" | "running" | "completed" | "failed" | "cancelled" | "approval_required";
  created_by: string;
  conversation_id: string;
  metadata?: Record<string, unknown>;
  plan?: Record<string, unknown>;
  result?: Record<string, unknown>;
  error?: string;
  replan_count: number;
  source: string;
  caller_task_id?: string;
  template_id?: string;
  template_version?: number;
  policy_applied?: string[];
  completed_subtasks: number;
  total_subtasks: number;
  agent_ids: string[];
  created_at: string;
  completed_at?: string;
}

export interface SubTask {
  id: string;
  task_id: string;
  agent_id: string;
  instruction: string;
  depends_on: string[];
  status: "pending" | "running" | "completed" | "failed" | "input_required" | "approval_required" | "cancelled" | "blocked";
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: string;
  a2a_task_id?: string;
  attempt: number;
  max_attempts: number;
  matched_skills?: string[];
  attempt_history?: Record<string, unknown>[];
  created_at: string;
  started_at?: string;
  completed_at?: string;
}

export interface TaskWithSubtasks extends Task {
  subtasks: SubTask[];
}

// Events

export interface TaskEvent {
  id: string;
  task_id: string;
  subtask_id?: string;
  type: string;
  actor_type: "system" | "agent" | "user";
  actor_id?: string;
  data: Record<string, unknown>;
  created_at: string;
}

// Messages (Group Chat)

export interface Message {
  id: string;
  task_id: string;
  conversation_id: string;
  sender_type: "agent" | "user" | "system";
  sender_id?: string;
  sender_name: string;
  content: string;
  mentions: string[];
  metadata?: Record<string, unknown>;
  created_at: string;
}

// Tool Call Events (SSE real-time visibility)
export interface ToolCallEvent {
  id: string;
  subtask_id: string;
  agent_id: string;
  agent_name?: string;
  tool_name: string;
  status: "started" | "completed";
  args?: string;
  summary?: string;
  created_at: string;
}

// Streaming Messages (Phase 9: Streaming Output)

export interface StreamingMessage {
  agent_id: string;
  agent_name: string;
  subtask_id: string;
  content: string; // Accumulated text so far
  started_at: string; // ISO timestamp of first delta
}

// Artifact Types (Phase 8: Artifact Rendering)

export interface SearchResult {
  title: string;
  url: string;
  snippet: string;
}

export interface SearchResultsArtifact {
  type: "search_results";
  title?: string;
  data: SearchResult[];
}

export interface CodeArtifact {
  type: "code";
  title?: string;
  data: {
    language: string;
    code: string;
    filename?: string;
  };
}

export interface TableArtifact {
  type: "table";
  title?: string;
  data: {
    headers: string[];
    rows: string[][];
  };
}

export interface DataArtifact {
  type: "data";
  title?: string;
  data: Record<string, unknown>;
}

export type Artifact =
  | SearchResultsArtifact
  | CodeArtifact
  | TableArtifact
  | DataArtifact;

export type ArtifactType = Artifact["type"];

export interface WorkflowTemplate {
  id: string;
  name: string;
  description: string;
  version: number;
  steps: Record<string, unknown>[];
  variables: Record<string, unknown>[];
  source_task_id?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
  usage_count?: number;
  success_rate?: number;
  avg_duration_sec?: number;
}

export interface Policy {
  id: string;
  name: string;
  rules: Record<string, unknown>;
  priority: number;
  is_active: boolean;
  created_at: string;
}

// Analytics

export interface DashboardData {
  total_tasks: number;
  completed_tasks: number;
  failed_tasks: number;
  running_tasks: number;
  success_rate: number;
  total_agents: number;
  online_agents: number;
  avg_duration_sec: number;
  status_distribution: { status: string; count: number }[];
  daily_tasks: { date: string; count: number }[];
  agent_usage: { id: string; name: string; task_count: number; completed: number; failed: number }[];
}

export interface AgentTaskDetail {
  id: string;
  task_title: string;
  status: string;
  duration_sec: number;
  created_at: string;
}

// Timeline

export interface TimelineEvent {
  id: string;
  task_id: string;
  subtask_id?: string;
  type: string;
  actor_type: string;
  actor_id?: string;
  data: Record<string, unknown>;
  created_at: string;
}

// Agent Health

export interface AgentHealthOverview {
  id: string;
  name: string;
  status: string;
  is_online: boolean;
  last_health_check: string | null;
  total_subtasks: number;
  completed: number;
  failed: number;
  success_rate: number;
  avg_duration_sec: number;
}

export interface AgentHealthDetail extends AgentHealthOverview {
  endpoint: string;
  skill_hash: string;
}

// Agent Activity Status (real-time, derived from SSE events)
export type AgentActivityStatus = "online" | "working" | "idle" | "offline" | "unknown";

// Audit Log

export interface AuditLogEntry {
  id: string;
  task_id: string;
  subtask_id?: string;
  type: string;
  actor_type: string;
  actor_id?: string;
  data: Record<string, unknown>;
  created_at: string;
}

export interface PaginatedAuditLogs {
  items: AuditLogEntry[];
  total: number;
  page: number;
  per_page: number;
  pages: number;
}

// Conversations

export interface Conversation {
  id: string;
  title: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface ConversationListItem {
  id: string;
  title: string;
  agent_count: number;
  task_count: number;
  latest_status: string;
  updated_at: string;
}

// Webhooks

export interface WebhookConfig {
  id: string;
  name: string;
  url: string;
  events: string[];
  is_active: boolean;
  secret: string;
  created_at: string;
}
