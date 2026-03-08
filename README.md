# Zynqel Core

Agent-agnostic runtime engine for CLI-based AI coding agents.

A single Go binary that runs CLI-based AI coding agents in isolated Docker sessions and streams interactive terminal output in real time.

Core is the **data plane**. It executes sessions, manages lifecycles, and streams PTY output. It knows nothing about users, billing, or multi-tenancy.

## Requirements

| Requirement | Version |
|---|---|
| Go | 1.22+ |
| Docker Engine | Latest |
| Docker Compose | Latest |

## Quick Start

```bash
# Build
go build -o bin/zynqel-core ./cmd/zynqel-core

# Run
./bin/zynqel-core

# Test
go test ./...
```

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

## Project Structure

```
zynqel-core/
├── cmd/zynqel-core/       # Entry point
├── internal/
│   ├── server/            # HTTP + WebSocket server
│   ├── session/           # Session manager, lifecycle
│   ├── sandbox/           # Sandbox interface + DockerSandbox
│   ├── adapter/           # AgentAdapter interface + ClaudeAdapter
│   ├── pty/               # PTY stream engine
│   ├── intercept/         # Intercepter engine
│   └── policy/            # Resource limits, timeouts
├── web/                   # Dev console (static HTML/JS)
├── docker/                # Session container Dockerfiles
└── scripts/               # Dev tooling
```

## GitHub Flow

```bash
# Start work on an issue
scripts/github-flow.sh start <issue-number> <feature|fix|refactor|bugfix>

# Open a PR
scripts/github-flow.sh pr <issue-number> [--draft]
```

Branch format: `<type>/ZYNQ-<number>_<slug>`

## License

AGPL-3.0
