export type SessionStatus = "pending" | "running" | "stopped" | "error";

export interface SessionSpec {
  agent: string;
  image?: string;
  workspace_id?: string;
  repo_url?: string;
  branch?: string;
  env?: Record<string, string>;
}

export interface Session {
  id: string;
  spec: SessionSpec;
  status: SessionStatus;
  container_id?: string;
  created_at: string;
  stopped_at?: string;
  error?: string;
}

export interface ContainerStats {
  cpu_percent: number;
  memory_mb: number;
  memory_max_mb: number;
}

export interface SystemInfo {
  max_sessions: number;
  active_count: number;
  memory_mb: number;
  cpu_quota: number;
  idle_timeout: number;
  hard_timeout: number;
}

export interface Workspace {
  id: string;
  created_at: string;
  image?: string;
  agent?: string;
}
