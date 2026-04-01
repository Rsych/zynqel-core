export type SessionStatus = "pending" | "running" | "stopped" | "error";

export interface SessionSpec {
  agent: string;
  image?: string;
  workspace_id?: string;
  repo_url?: string;
  branch?: string;
  git_token?: string;
  ssh_key_path?: string;
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

export interface AgentConfig {
  name: string;
  builtin: boolean;
  command?: string[];
  image?: string;
  dockerfile?: string;
  env?: Record<string, string>;
}

export interface Workspace {
  id: string;
  created_at: string;
  image?: string;
  agent?: string;
}

export interface SessionRecord {
  id: string;
  workspace_id?: string;
  agent: string;
  image?: string;
  repo_url?: string;
  branch?: string;
  status: string;
  created_at: string;
  stopped_at?: string;
  duration?: string;
  error?: string;
  has_log: boolean;
}
