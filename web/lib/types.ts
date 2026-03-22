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
