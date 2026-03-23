// web/lib/types.ts — TypeScript interfaces matching Go models

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
  agent_card_url: string;
  agent_card?: Record<string, unknown>;
  card_fetched_at?: string;
  capabilities: string[];
  skills?: AgentSkill[];
  status: "active" | "inactive" | "degraded";
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
  status: "pending" | "running" | "completed" | "failed" | "input_required" | "cancelled" | "blocked";
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: string;
  a2a_task_id?: string;
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
