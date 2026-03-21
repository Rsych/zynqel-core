# Zynqel Core

Run AI coding agents in isolated Docker sessions with real-time terminal streaming.

Zynqel Core is an agent-agnostic runtime engine — a single Go binary that manages Docker containers, streams PTY output over WebSocket, and persists workspaces across restarts. Works with any CLI-based AI coding tool (Claude Code, Qwen Code, Aider, etc.).

## Quick Start

```bash
# 1. Build Docker images
make images

# 2. Build and run
make run

# 3. Open browser
open http://localhost:8080/console/
```

Or with Docker Compose:

```bash
make images
docker compose up
```

## Requirements

- Go 1.22+
- Docker Engine

## Features

- **Agent-agnostic** — plug in any CLI coding tool (Claude, Qwen, or bare shell)
- **Isolated sessions** — each session runs in its own Docker container
- **Real-time PTY streaming** — full interactive terminal in the browser via WebSocket
- **Workspace persistence** — code (Docker volumes) and installed packages (Docker commit) survive restarts
- **Prompt interception** — CLI confirmation prompts (Y/n) rendered as UI buttons
- **Multi-viewer** — multiple browser tabs can watch the same session
- **Reconnect with replay** — 64KB ring buffer replays recent output on reconnect
- **Session lifecycle** — idle timeout, hard timeout, concurrency cap
- **Web dev console** — built-in terminal UI with workspace management

## Configuration

All settings via environment variables:

```env
ZYNQEL_PORT=8080                    # Server port (default: 8080)
ZYNQEL_MAX_SESSIONS=10             # Max concurrent sessions (default: 10, 0=unlimited)
ZYNQEL_IDLE_TIMEOUT=900            # Idle timeout in seconds (default: 900 = 15min)
ZYNQEL_HARD_TIMEOUT=1800           # Hard timeout in seconds (default: 1800 = 30min)
ZYNQEL_SESSION_MEMORY_MB=512       # Container memory limit (default: 512MB)
ZYNQEL_SESSION_CPU_QUOTA=100       # CPU quota, 100 = 1 core (default: 100)
```

## API

### HTTP

```
GET    /health                Health check
POST   /sessions              Create session
GET    /sessions               List sessions
GET    /sessions/:id           Session details
DELETE /sessions/:id           Stop session
GET    /workspaces             List saved workspaces
DELETE /workspaces/:id         Delete workspace
GET    /console/               Web terminal UI
```

### WebSocket

`WS /sessions/:id/stream`

```json
// Server → Client
{"type": "pty.output", "data": "<base64>"}
{"type": "session.state", "data": "running|stopped"}
{"type": "intercept.event", "data": {"id": "evt_...", "text": "Allow?", "options": ["Yes","No"]}}

// Client → Server
{"type": "pty.input", "data": "<base64>"}
{"type": "pty.resize", "data": {"cols": 80, "rows": 24}}
{"type": "intercept.response", "data": {"id": "evt_...", "option": "Yes"}}
```

### Create Session

```bash
curl -X POST http://localhost:8080/sessions \
  -H 'Content-Type: application/json' \
  -d '{
    "agent": "shell",
    "image": "zynqel-qwen:latest",
    "workspace_id": "my-project",
    "repo_url": "https://github.com/user/repo.git",
    "branch": "main"
  }'
```

## Docker Images

```
zynqel-base     Node.js, git, python3, vim, ripgrep, build tools
├── zynqel-claude   + Claude Code CLI
└── zynqel-qwen     + Qwen Code CLI
```

Build all: `make images`

## Workspace Persistence

Workspaces persist across session restarts and server reboots:

- **Code** — Docker volume mounted at `/workspace`
- **Environment** — `docker commit` saves installed packages (npm, pip, apt)
- **Resume** — same `workspace_id` = pick up where you left off

## Architecture

```
Browser (xterm.js)
    ↕ WebSocket
Zynqel Core (Go binary)
    ↕ Docker API
Containers (1 per session)
```

Core is the **data plane** — it handles containers and PTY. It knows nothing about users, billing, or teams. That's [Zynqel Cloud](https://github.com/Rsych/zynqel-cloud) (SaaS layer, separate repo).

## Development

```bash
make build          # Build binary
make test           # Run tests with race detector
make lint           # Format + vet + golangci-lint
make images         # Build Docker images
make run            # Build + run
make clean          # Remove containers, volumes, images
```

## License

AGPL-3.0 — free to self-host. Building a SaaS on top requires open-sourcing your code.
