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

### Pre-push CI check — ALWAYS run before committing/pushing

Run the same checks as GitHub Actions CI to catch failures early:

```bash
gofmt -s -l .                                           # Format check (should print nothing)
go vet ./...                                            # Static analysis
$(go env GOPATH)/bin/golangci-lint run ./...             # Lint (golangci-lint v2.11.3)
go test -race ./...                                     # Tests with race detector
go build ./...                                          # Build
```

Install golangci-lint if missing:
```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.3
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

## Git & GitHub Flow

- Branch format: `<type>/ZYNQ-<number>_<slug>` (e.g., `feature/ZYNQ-01_http_server`)
- Types: `feature`, `fix`, `refactor`, `bugfix`
- Commit format: `<type>: <short description>` (e.g., `feat: add health endpoint`)
- Commit types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`
- Base branch: `master`

### GitHub Flow Script — ALWAYS use this

**Starting work on an issue:**
```bash
scripts/github-flow.sh start <issue-number> <feature|fix|refactor|bugfix>
# Example: scripts/github-flow.sh start 5 feature
# Creates branch: feature/ZYNQ-05_session_spec_session_struct_in_memory_registry_crud_api
# Sets project status to "In Progress"
```

**Opening a PR:**
```bash
scripts/github-flow.sh pr <issue-number> [--draft]
# Example: scripts/github-flow.sh pr 5
# Pushes branch, creates PR linked to issue, updates project status
```

**Rules:**
- ALWAYS use `scripts/github-flow.sh` to start branches and open PRs — never manually
- The script enforces branch naming, links issues, and updates the GitHub Project board
- Working tree must be clean before `start` (commit or stash first)
- Requires: `gh`, `git`, `jq`
- Override project number: `PROJECT_NUMBER=5 scripts/github-flow.sh start 5 feature`

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
