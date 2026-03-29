# Contributing to Zynqel Core

Thanks for your interest in contributing! Zynqel Core is the open-source runtime engine that powers Zynqel — here's how to get involved.

## Getting Started

1. Fork the repository
2. Clone your fork and install dependencies:
   ```bash
   git clone https://github.com/<your-username>/zynqel-core.git
   cd zynqel-core
   make web-install
   ```
3. Make sure everything builds and passes:
   ```bash
   make lint
   make test
   make build
   ```

## Development Workflow

```bash
# Run in development mode (Go API + Next.js hot-reload)
make dev

# Run full CI checks locally before pushing
make lint && make test
```

### Branch Naming

```
<type>/short-description
```

Types: `feature`, `fix`, `refactor`, `bugfix`

### Commit Messages

```
<type>: <short description>
```

Types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`

Examples:
- `feat: add session timeout configuration`
- `fix: prevent goroutine leak on rapid session delete`
- `docs: add WebSocket protocol examples`

## Pull Requests

1. Create a branch from `master`
2. Make your changes — keep PRs focused on a single concern
3. Run the full CI check suite locally:
   ```bash
   gofmt -s -l .                           # Should print nothing
   go vet ./...
   golangci-lint run ./...
   go test -race ./...
   go build ./...
   cd web && npm run build
   ```
4. Open a PR against `master`
5. Describe what changed and why in the PR description

## Code Conventions

- **Go 1.22+** — prefer the standard library
- **Error handling:** return errors, don't panic. Wrap with `fmt.Errorf("context: %w", err)`
- **Naming:** Go conventions — `PascalCase` for exported, `camelCase` for unexported
- **Interfaces:** define in the package that *uses* them
- **No premature abstraction** — make it work, then make it right

## Architecture Boundaries

Zynqel Core is the **data plane only**. Keep these concerns out:

- No user management or authentication
- No billing or subscription logic
- No multi-tenancy
- No Kubernetes or multi-host distribution

These belong in [Zynqel Cloud](https://github.com/Rsych/zynqel-cloud) (the SaaS layer).

## Reporting Bugs

Open a [GitHub issue](https://github.com/Rsych/zynqel-core/issues) with:
- What you expected vs what happened
- Steps to reproduce
- Go version, Docker version, OS

## Security Issues

Please report security vulnerabilities privately — see [SECURITY.md](SECURITY.md).

## License

By contributing, you agree that your contributions will be licensed under the [AGPL-3.0 license](LICENSE).
