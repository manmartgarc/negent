# Agent Sync Types

This document records Negent's sync-type taxonomy for each supported agent and links to the upstream source of truth used to define it.

## Claude

Upstream reference: <https://code.claude.com/docs/en/claude-directory>

Negent syncs these Claude-specific type IDs:

- `claude-md`: `CLAUDE.md`
- `rules`: `rules/*.md`
- `commands`: `commands/*.md`
- `skills`: `skills/<name>/`
- `agents`: `agents/*.md`
- `plugins`: `plugins/<name>/`
- `output-styles`: `output-styles/*.md`
- `agent-memory`: `agent-memory/<name>/`
- `auto-memory`: `projects/<project>/memory/`
- `sessions`: `projects/<project>/*.jsonl`
- `history`: `history.jsonl`
- `keybindings`: `keybindings.json`

Default Claude sync types:

- `claude-md`
- `rules`
- `commands`
- `skills`
- `agents`
- `plugins`
- `output-styles`
- `agent-memory`
- `auto-memory`

Negent intentionally does not sync these Claude files in this phase:

- `settings.json`
- `settings.local.json`
- `~/.claude.json`
- `.mcp.json`
- `.worktreeinclude`
- Credentials, auth files, caches, telemetry, and other transient directories excluded by `internal/agent/claude`

Legacy Claude config aliases still accepted on read:

- `config` -> `claude-md`
- `custom-code` -> `commands`, `skills`, `agents`, `rules`, `output-styles`
- `memory` -> `agent-memory`, `auto-memory`
- `sessions` -> `sessions`
- `history` -> `history`
- `plugins` -> `plugins`
