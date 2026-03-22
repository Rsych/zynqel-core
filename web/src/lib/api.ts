import type {
  Session,
  SessionSpec,
  ContainerStats,
  SystemInfo,
  Workspace,
  AgentConfig,
} from "./types";

// In dev, Next.js runs on :3000 but Go API is on :8080.
// In production (static export served by Go), same origin.
const API_BASE =
  process.env.NEXT_PUBLIC_API_URL ||
  (typeof window !== "undefined" && window.location.port === "3000"
    ? "http://localhost:8080"
    : "");

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export const api = {
  // Sessions
  listSessions: () => fetchJSON<Session[]>("/sessions"),
  getSession: (id: string) => fetchJSON<Session>(`/sessions/${id}`),
  createSession: (spec: SessionSpec) =>
    fetchJSON<Session>("/sessions", {
      method: "POST",
      body: JSON.stringify(spec),
    }),
  stopSession: (id: string) =>
    fetchJSON<Session>(`/sessions/${id}/stop`, { method: "POST" }),
  deleteSession: (id: string) =>
    fetchJSON<void>(`/sessions/${id}`, { method: "DELETE" }),
  getSessionStats: (id: string) =>
    fetchJSON<ContainerStats>(`/sessions/${id}/stats`),

  // Agents
  listAgents: () => fetchJSON<AgentConfig[]>("/agents"),
  createAgent: (cfg: Omit<AgentConfig, "builtin">) =>
    fetchJSON<AgentConfig>("/agents", {
      method: "POST",
      body: JSON.stringify(cfg),
    }),
  updateAgent: (name: string, cfg: Omit<AgentConfig, "builtin">) =>
    fetchJSON<AgentConfig>(`/agents/${name}`, {
      method: "PUT",
      body: JSON.stringify(cfg),
    }),
  deleteAgent: (name: string) =>
    fetchJSON<void>(`/agents/${name}`, { method: "DELETE" }),

  // System
  getSystemInfo: () => fetchJSON<SystemInfo>("/system/info"),

  // Workspaces
  listWorkspaces: () => fetchJSON<Workspace[]>("/workspaces"),
  deleteWorkspace: (id: string) =>
    fetchJSON<void>(`/workspaces/${id}`, { method: "DELETE" }),

  // WebSocket URL for session stream
  streamURL: (sessionId: string) => {
    const wsBase =
      API_BASE.replace(/^http/, "ws") ||
      `ws://${typeof window !== "undefined" ? window.location.host : "localhost:8080"}`;
    return `${wsBase}/sessions/${sessionId}/stream`;
  },
};
