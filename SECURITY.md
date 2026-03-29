# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Zynqel Core, please report it responsibly.

Please open a [GitHub issue](https://github.com/Rsych/zynqel-core/issues) with:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

## Scope

This policy covers the Zynqel Core repository (`zynqel-core`). This includes:

- Go backend (API server, session management, sandbox)
- Docker container management
- WebSocket PTY streaming
- Web dashboard

## Known Security Considerations

- **WebSocket origins:** Zynqel Core accepts WebSocket connections from any origin by default. This is intentional for self-hosted deployments. If exposing to the internet, place it behind a reverse proxy with proper origin restrictions.
- **No authentication:** Core has no built-in auth. It's designed to run behind a trusted network boundary or an authenticating reverse proxy.
- **Docker socket access:** Core requires access to the Docker socket to manage containers. Run it with appropriate permissions.
- **Environment variables:** API responses redact env var values, but they are stored in memory in plaintext during the session lifecycle.

## Supported Versions

We release security fixes for the latest version only.
