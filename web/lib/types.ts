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
