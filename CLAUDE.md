# CLAUDE.md — Zynqel Core

## Project

Zynqel Core is an agent-agnostic runtime engine that runs CLI-based AI coding agents in isolated Docker sessions with real-time PTY streaming. Written in Go.

## Build & Test

```bash
go build ./...                          # Build all packages
go build -o bin/zynqel-core ./cmd/zynqel-core  # Build binary
go test ./...                           # Run all tests
go vet ./...                            # Static analysis
```

## Code Conventions

- **Go 1.22+** — use standard library where possible
- **Package layout:** `cmd/` for entry points, `internal/` for private packages
- **Naming:** Go conventions — exported names are `PascalCase`, unexported are `camelCase`
- **Error handling:** Return errors, don't panic. Wrap with `fmt.Errorf("context: %w", err)`
- **Interfaces:** Define in the package that *uses* them, not the package that implements them
- **No premature abstraction** — make it work, then make it right

## Architecture Rules

1. Docker only — no MicroVM
2. Single host only — no distribution
3. No billing — that's Cloud
4. No Kubernetes
5. Core is the data plane — no user management, no multi-tenancy

## Git

- Branch format: `<type>/ZYNQ-<number>_<slug>` (e.g., `feature/ZYNQ-01_http_server`)
- Types: `feature`, `fix`, `refactor`, `bugfix`
- Commit format: `<type>: <short description>` (e.g., `feat: add health endpoint`)
- Commit types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`
- Base branch: `master`

## Key Interfaces

```go
// Sandbox — pluggable execution backend
type Sandbox interface {
    StartSession(spec SessionSpec) error
    AttachPTY() (PTYStream, error)
    StreamOutput() (<-chan []byte, error)
    InjectInput([]byte) error
    KillSession() error
    Cleanup() error
}

// AgentAdapter — pluggable agent support
type AgentAdapter interface {
    Start(workspace string) error
    HandleInput([]byte) error
    Stop() error
}
```

## API Surface

- `GET /health` — health check
- `POST /sessions` — create session
- `GET /sessions` — list sessions
- `GET /sessions/:id` — session details
- `DELETE /sessions/:id` — kill session
- `WS /sessions/:id/stream` — real-time PTY stream
