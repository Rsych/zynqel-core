# Zynqel Core — Project Plan

**Agent-agnostic runtime engine for CLI-based AI coding agents**

License: AGPL-3.0 | Language: Go

---

## What Is Core

A single Go binary that runs CLI-based AI coding agents in isolated Docker sessions and streams interactive terminal output in real time.

Core is the **data plane**. It executes sessions, manages lifecycles, and streams PTY output. It knows nothing about users, billing, or multi-tenancy.

---

## Status: Complete ✅

All 3 epics delivered. 23/23 issues closed. 45 PRs merged.

| Epic | Status | Tag |
|------|--------|-----|
| Core Skeleton + Docker Control | ✅ Done | v0.1.0 |
| Claude Adapter Integration | ✅ Done | v0.2.0 |
| Intercepter + Lifecycle | ✅ Done | v0.3.0 |
| Workspace Persistence | ✅ Done | v0.4.0 |

---

## Scope

### IN Core

| Component | Description | Status |
|---|---|---|
| HTTP Control API | Sessions + workspaces CRUD | ✅ |
| WebSocket Stream API | Real-time PTY streaming | ✅ |
| Session Manager | Create, track, kill, cleanup | ✅ |
| Sandbox Abstraction | `Sandbox` interface — Docker backend | ✅ |
| Agent Adapter Layer | `AgentAdapter` interface — Claude, extensible | ✅ |
| PTY Stream Engine | Attach, read, write pseudo-terminal | ✅ |
| Output Broadcaster | Ring buffer, multi-viewer, WS reconnect replay | ✅ |
| Intercepter Engine | Detect CLI prompts → structured events + UI buttons | ✅ |
| Resource Policy | CPU/memory limits, idle/hard timeout, max sessions | ✅ |
| Workspace Persistence | Docker volumes (code) + commit (env) | ✅ |
| Web Dev Console | Tailwind, xterm.js, workspace-centric UI | ✅ |
| Docker Images | zynqel-base, zynqel-claude, zynqel-qwen | ✅ |
| Orphan Cleanup | Sweep stale containers on boot | ✅ |

### NOT in Core

| Component | Where |
|---|---|
| Authentication / user management | Cloud |
| Billing / usage metering | Cloud |
| Multi-tenant orchestration | Cloud |
| Runner pool / distributed scheduling | Cloud |
| Team / org / permissions | Cloud |
| Production UI (Next.js) | Cloud |
| Image registry push | Cloud |

---

## API

### HTTP

| Method | Path | Description |
|---|---|---|
| GET | `/health` | Health check |
| POST | `/sessions` | Create session (reuses existing for same workspace) |
| GET | `/sessions` | List sessions |
| GET | `/sessions/:id` | Get session details |
| DELETE | `/sessions/:id` | Kill session (commits workspace image) |
| GET | `/workspaces` | List saved workspaces |
| DELETE | `/workspaces/:id` | Delete workspace volume + image |
| GET | `/console/` | Web dev console |

### WebSocket

`WS /sessions/:id/stream`

| Message Type | Direction | Description |
|---|---|---|
| `pty.output` | Server → Client | Terminal output (base64) |
| `pty.input` | Client → Server | Keyboard input (base64) |
| `pty.resize` | Client → Server | Terminal dimensions |
| `session.state` | Server → Client | Lifecycle updates |
| `intercept.event` | Server → Client | Detected CLI prompt |
| `intercept.response` | Client → Server | User's prompt response |
| `error` | Server → Client | Runtime errors |

### SessionSpec (POST /sessions body)

```json
{
  "agent": "shell|claude|qwen",
  "image": "custom-image:tag",
  "workspace_id": "my-project",
  "repo_url": "https://github.com/user/repo.git",
  "branch": "main",
  "env": {"KEY": "value"}
}
```

---

## Configuration

```env
ZYNQEL_PORT=8080                    # Server port (default: 8080)
ZYNQEL_MAX_SESSIONS=10             # Max concurrent sessions (default: 10, 0=unlimited)
ZYNQEL_IDLE_TIMEOUT=900            # Idle timeout seconds (default: 900, 0=disabled)
ZYNQEL_HARD_TIMEOUT=1800           # Hard timeout seconds (default: 1800, 0=disabled)
ZYNQEL_SESSION_MEMORY_MB=512       # Container memory limit (default: 512)
ZYNQEL_SESSION_CPU_QUOTA=100       # CPU quota percentage (default: 100 = 1 core)
```

---

## Remaining Polish (TODO)

- [ ] README.md with setup instructions
- [ ] docker-compose.yml for one-command launch
- [ ] Fix: loading states for workspace create/stop/open
- [ ] Fix: welcome/onboarding screen refinement
- [ ] Tag v0.4.0 release
- [ ] Makefile for common commands (build, test, images)

---

## Rules

1. Docker only — no MicroVM
2. Single host only — no distribution
3. No billing — that's Cloud
4. No Kubernetes
5. Core is the data plane — no user management, no multi-tenancy
6. Keep Cloud concerns out of Core
7. Plan before implementing non-trivial tasks
8. Run full test suite before pushing
