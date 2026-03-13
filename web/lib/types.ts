export interface Agent {
  id: string;
  name: string;
  description: string;
  system_prompt: string;
  capabilities: string[];
  color: string;
  created_at: string;
}

export interface Task {
  id: string;
  title: string;
  description: string;
  status: "pending" | "running" | "completed" | "failed";
  created_at: string;
  completed_at?: string;
}

export interface Channel {
  id: string;
  task_id: string;
  document: string;
  status: "active" | "archived";
  created_at: string;
}

export interface Message {
  id: string;
  channel_id: string;
  sender_id: string;
  sender_name: string;
  content: string;
  type: "text" | "result" | "system";
  created_at: string;
}

export interface ChannelAgent {
  channel_id: string;
  agent_id: string;
  status: "idle" | "working" | "done";
}

export interface ChannelDetail {
  channel: Channel;
  messages: Message[];
  agents: ChannelAgent[];
}

export type SSEEventType =
  | "task_started"
  | "channel_created"
  | "agent_joined"
  | "agent_working"
  | "message"
  | "document_updated"
  | "agent_done"
  | "task_completed";

export interface SSEEvent<T = unknown> {
  type: SSEEventType;
  data: T;
}
