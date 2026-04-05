# AGENTS.md

Compatibility note for agent tooling that auto-reads `AGENTS.md`.

Canonical guidance now lives in `CLAUDE.md`.
If there is any conflict, follow `CLAUDE.md`.
The sections below are intentionally abbreviated reminders only.

## Workflow Script

Use the repository workflow helper:

```bash
scripts/github-flow.sh start <issue-number> <feature|fix|refactor|bugfix>
scripts/github-flow.sh pr <issue-number> [--draft]
```