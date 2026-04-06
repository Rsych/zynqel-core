# CLAUDE.md — Zynqel Core

## Project

Zynqel Core is an agent-agnostic runtime engine that runs CLI-based AI coding agents in isolated Docker sessions with real-time PTY streaming. Written in Go.

### Product Context

Zynqel has two layers — **Core is the open-source data plane only:**

- **Zynqel Core (AGPL v3)** — this repo. Runs agents in Docker, streams PTY, manages sessions. No auth, no billing, no multi-tenancy.
- **Zynqel Cloud (Proprietary)** — SaaS layer built on top. Adds multi-tenancy, billing, mobile UI, push notifications, team management. Built with Next.js.

**Architecture boundary:** Core handles containers and PTY. Everything else (users, orgs, billing, UI) belongs in Cloud. When making decisions in Core, keep this separation clean.

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

### Docker images

```bash
docker build -t zynqel-claude:latest images/claude/     # Build Claude agent image
docker build -t zynqel-opencode:latest images/opencode/ # Build OpenCode agent image
docker build -t zynqel-codex:latest images/codex/       # Build Codex agent image
```

## Workflow Rules

### Plan before coding
- Enter plan mode for ANY non-trivial task (3+ steps or architectural decisions)
- If something goes sideways mid-implementation, STOP and re-plan — don't keep pushing
- Get user sign-off on the plan before writing code

### Verify before done
- Always run the FULL test suite (`go test -race ./...`), not just the new package
- Run CI checks before committing — never push without passing all checks
- Smoke test with real Docker when changing session/container/PTY code
- Never mark a task complete without proving it works

### Autonomous bug fixing
- When CI fails or a bug is reported: fix it, don't ask for hand-holding
- Point at logs, errors, failing tests — then resolve them

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
6. Keep Cloud concerns out of Core — auth, orgs, billing, UI belong in Cloud

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

## Key Interfaces (actual, not aspirational)

```go
// Sandbox — pluggable execution backend (internal/sandbox/sandbox.go)
type Sandbox interface {
    Create(ctx context.Context, spec Spec) (string, error)
    Start(ctx context.Context, id string) error
    Stop(ctx context.Context, id string) error
    Remove(ctx context.Context, id string) error
    Attach(ctx context.Context, id string) (PTYConn, error)
    Exec(ctx context.Context, id string, cmd []string) (PTYConn, error)
    ExecRun(ctx context.Context, id string, cmd []string) ([]byte, error)
}

// AgentAdapter — pluggable agent support (internal/adapter/adapter.go)
type AgentAdapter interface {
    Image() string
    Start(ctx context.Context, containerID string) (sandbox.PTYConn, error)
    Stop() error
}
```

## API Surface

- `GET /health` — health check
- `POST /sessions` — create session (`{"agent": "claude|opencode|codex|shell", "repo_url": "...", "branch": "..."}`)
- `GET /sessions` — list sessions
- `GET /sessions/:id` — session details
- `DELETE /sessions/:id` — kill session (graceful SIGTERM → SIGKILL)
- `WS /sessions/:id/stream` — real-time PTY stream (base64-encoded I/O)
- `GET /console/` — web dev console (xterm.js)

## Session Lifecycle

```
POST /sessions → Create container → Start container → Clone repo (if repo_url)
    → Start agent adapter (if not shell) → Session running

DELETE /sessions/:id → adapter.Stop() (SIGTERM→SIGKILL) → container Stop → container Remove

Server SIGINT/SIGTERM → HTTP drain (10s) → Shutdown all sessions (30s)
Boot → Sweep orphan containers (zynqel.managed=true)
```
