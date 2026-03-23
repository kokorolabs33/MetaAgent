// web/lib/types.ts — V2 types matching Go models

export interface Organization {
  id: string;
  name: string;
  slug: string;
  plan: string;
  settings: Record<string, unknown>;
  budget_usd_monthly?: number;
  budget_alert_threshold: number;
  created_at: string;
}

export interface OrgListItem {
  id: string;
  name: string;
  slug: string;
  plan: string;
  created_at: string;
}

export interface User {
  id: string;
  email: string;
  name: string;
  avatar_url: string;
  created_at: string;
}

export interface OrgMember {
  org_id: string;
  user_id: string;
  role: "owner" | "admin" | "member" | "viewer";
  joined_at: string;
}

export interface OrgMemberWithUser {
  id: string;
  email: string;
  name: string;
  avatar_url: string;
  role: "owner" | "admin" | "member" | "viewer";
  joined_at: string;
}

export interface PageResponse<T> {
  items: T[];
  next_cursor: string | null;
  has_more: boolean;
}

// Agent Registry

export interface Agent {
  id: string;
  org_id: string;
  name: string;
  version: string;
  description: string;
  endpoint: string;
  adapter_type: "http_poll" | "native";
  adapter_config?: Record<string, unknown>;
  auth_type: "none" | "bearer" | "api_key" | "basic";
  capabilities: string[];
  status: "active" | "inactive" | "degraded";
  created_at: string;
  updated_at: string;
}

// Task System

export interface Task {
  id: string;
  org_id: string;
  title: string;
  description: string;
  status: "pending" | "planning" | "running" | "completed" | "failed" | "cancelled";
  created_by: string;
  metadata?: Record<string, unknown>;
  plan?: Record<string, unknown>;
  result?: Record<string, unknown>;
  error?: string;
  replan_count: number;
  created_at: string;
  completed_at?: string;
}

export interface SubTask {
  id: string;
  task_id: string;
  agent_id: string;
  instruction: string;
  depends_on: string[];
  status: "pending" | "running" | "completed" | "failed" | "waiting_for_input" | "cancelled" | "blocked";
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: string;
  poll_job_id?: string;
  attempt: number;
  max_attempts: number;
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
  sender_type: "agent" | "user" | "system";
  sender_id?: string;
  sender_name: string;
  content: string;
  mentions: string[];
  metadata?: Record<string, unknown>;
  created_at: string;
}
