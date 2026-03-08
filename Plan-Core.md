# Zynqel Core — Project Plan

**Agent-agnostic runtime engine for CLI-based AI coding agents**

License: AGPL-3.0 | Language: Go | Repo: Private → Public after Week 1

---

## What Is Core

A single Go binary that runs CLI-based AI coding agents in isolated Docker sessions and streams interactive terminal output in real time.

Core is the **data plane**. It executes sessions, manages lifecycles, and streams PTY output. It knows nothing about users, billing, or multi-tenancy.

---

## Scope

### IN Core

| Component | Description |
|---|---|
| HTTP Control API | `POST/GET/DELETE /sessions`, `/health` |
| WebSocket Stream API | `WS /sessions/:id/stream` — real-time PTY streaming |
| Session Manager | Create, track, kill, cleanup sessions |
| SessionSpec | Stable contract: agent, repo, env, resources, timeouts |
| Sandbox Abstraction | `Sandbox` interface — pluggable execution backend |
| DockerSandbox | One container per session |
| Agent Adapter Layer | `AgentAdapter` interface — pluggable agent support |
| ClaudeAdapter | Launches Claude CLI in PTY |
| PTY Stream Engine | Attach, read, write to pseudo-terminal |
| Intercepter Engine | Detect CLI prompts → structured runtime events |
| Resource Policy | CPU/memory limits, idle/hard timeout, max sessions |
| Web Dev Console | Minimal HTML/JS terminal UI |
| Orphan Cleanup | Sweep stale containers on boot |

### NOT in Core

| Component | Where |
|---|---|
| Authentication / user management | Cloud |
| Billing / usage metering | Cloud |
| Multi-tenant orchestration | Cloud |
| Runner pool / distributed scheduling | Cloud |
| Team / org / permissions | Cloud |
| Dashboard UI | Cloud |
| MicroVM sandbox | Post-MVP |
| Suspend / Resume | Post-MVP |

---

## Architecture

```
             ┌────────────────────────┐
             │   Web Dev Console      │
             │   (HTML/JS, minimal)   │
             └────────────┬───────────┘
                          │ WebSocket / HTTP
                          ▼
             ┌────────────────────────┐
             │     Zynqel Core        │
             │  (Single Go Binary)    │
             ├────────────────────────┤
             │ HTTP Control API       │
             │ WebSocket Stream API   │
             │------------------------│
             │ Session Manager        │
             │ Sandbox Abstraction    │
             │ Agent Adapter Layer    │
             │ PTY Stream Engine      │
             │ Intercepter Engine     │
             │ Resource Policy        │
             └────────────┬───────────┘
                          │
                          ▼
                   DockerSandbox
              (1 container per session)
```

---

## Directory Structure

```
zynqel-core/
├── cmd/
│   └── zynqel-core/
│       └── main.go
├── internal/
│   ├── server/          # HTTP + WebSocket server
│   ├── session/         # Session manager, SessionSpec, lifecycle
│   ├── sandbox/         # Sandbox interface + DockerSandbox
│   ├── adapter/         # AgentAdapter interface + ClaudeAdapter
│   ├── pty/             # PTY stream engine
│   ├── intercept/       # Intercepter engine
│   └── policy/          # Resource limits, timeouts
├── web/                 # Dev console (static HTML/JS)
├── docker/              # Dockerfiles for session containers
├── docker-compose.yml
├── Makefile
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

---

## Stable Contracts

### Sandbox Interface

```go
type Sandbox interface {
    StartSession(spec SessionSpec) error
    AttachPTY() (PTYStream, error)
    StreamOutput() (<-chan []byte, error)
    InjectInput([]byte) error
    KillSession() error
    Cleanup() error
}
```

### AgentAdapter Interface

```go
type AgentAdapter interface {
    Start(workspace string) error
    HandleInput([]byte) error
    Stop() error
}
```

### SessionSpec

```go
type SessionSpec struct {
    Agent            string
    RepoURL          string
    Branch           string
    Env              map[string]string
    StartupCommands  []string
    CPUQuota         int
    MemoryMB         int
    IdleTimeoutSec   int
    HardTimeoutSec   int
}
```

---

## API

### HTTP

| Method | Path | Description |
|---|---|---|
| GET | `/health` | Health check |
| POST | `/sessions` | Create session |
| GET | `/sessions` | List sessions |
| GET | `/sessions/:id` | Get session details |
| DELETE | `/sessions/:id` | Kill and cleanup session |

### WebSocket

`WS /sessions/:id/stream`

| Message Type | Direction | Description |
|---|---|---|
| `pty.output` | Server → Client | Raw terminal output |
| `pty.input` | Client → Server | User keyboard input |
| `session.state` | Server → Client | Lifecycle updates |
| `intercept.event` | Server → Client | Structured prompt events |
| `error` | Server → Client | Runtime errors |

---

## Milestones

### Week 1 — Core Skeleton + Docker Control

**Goal:** Browser shows bash output from a Docker container

| Day | Task | Done |
|---|---|---|
| 1 | Go project init, HTTP server, `/health` | |
| 2 | SessionSpec, Session struct, in-memory registry, CRUD API | |
| 3 | DockerSandbox — `docker run`, stop, remove | |
| 4 | Resource limits, container labels, orphan sweep on boot | |
| 5 | PTY attach, stdout stream, WebSocket endpoint | |
| 6 | Web dev console — HTML/JS, WS connect, render, input | |
| 7 | Cleanup: PTY teardown, graceful shutdown, race fixes | |

**Tag:** `v0.1.0` → Go public

---

### Week 2 — Claude Adapter Integration

**Goal:** Claude CLI runs in browser via Docker session

| Day | Task | Done |
|---|---|---|
| 8 | AgentAdapter interface, ClaudeAdapter, Start() | |
| 9 | PTY binding for Claude, input forwarding | |
| 10 | Repo clone, branch selection, startup commands | |
| 11 | Graceful Stop(), child process kill | |
| 12 | Hard kill test — long task → session kill | |
| 13 | WS reconnect — recent output buffer, state sync | |

**Tag:** `v0.2.0`

---

### Week 3 — Intercepter + Lifecycle

**Goal:** CLI confirms become UI buttons, sessions auto-expire

| Day | Task | Done |
|---|---|---|
| 14 | Intercepter — detect `(Y/n)`, `[y/N]`, `? Continue` | |
| 15 | `intercept.event` WebSocket messages | |
| 16 | UI confirm button → stdin injection | |
| 17 | Idle timeout (15 min → terminate) | |
| 18 | Hard timeout (30 min → force kill) | |
| 19 | Concurrency cap (MAX_SESSIONS) | |
| 20-21 | Stability: 10 concurrent sessions, leak check | |

**Tag:** `v0.3.0`

---

## Requirements

| Requirement | Version |
|---|---|
| Go | 1.22+ |
| Docker Engine | Latest |
| Docker Compose | Latest |

### Session Container Base Image

- Node.js (for Claude CLI)
- Git
- curl
- Claude CLI pre-installed

---

## Configuration

```env
ZYNQEL_PORT=8080
ZYNQEL_SANDBOX=docker
ZYNQEL_MAX_SESSIONS=10
ZYNQEL_IDLE_TIMEOUT=900
ZYNQEL_HARD_TIMEOUT=1800
ZYNQEL_SESSION_MEMORY_MB=512
ZYNQEL_SESSION_CPU_QUOTA=100
```

---

## GitHub Flow

### Branches

- `master` — stable, always works
- `feature/<name>` — features
- `fix/<name>` — bug fixes

### Commit Convention

```
<type>: <short description>

Types: feat, fix, refactor, docs, test, chore
```

### Releases

- `v0.1.0` — Week 1 (Docker + bash)
- `v0.2.0` — Week 2 (Claude adapter)
- `v0.3.0` — Week 3 (Intercepter + lifecycle)
- `v1.0.0` — Public launch ready

---

## Failure Conditions

If any of these happen, stop and fix:

- Zombie processes after session kill
- Container leak (not removed)
- WebSocket reconnect panic
- Claude exit leaves PTY hanging
- Docker restart orphans sessions
- Memory leak under load

---

## Rules

1. Docker only — no MicroVM
2. Single host only — no distribution
3. No billing — that's Cloud
4. No Kubernetes
5. No premature abstraction
6. Make it work, then make it right
