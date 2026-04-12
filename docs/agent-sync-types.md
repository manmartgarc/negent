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

### Project matching during pull

Claude stores per-project data (auto-memory, sessions) under `~/.claude/projects/<encoded-path>/`, where the directory name is derived from the project's absolute path on that machine. The same repo checked out at `/home/alice/repos/myapp` on one machine and `/Users/bob/code/myapp` on another produces two different encoded directory names.

During `pull`, negent must decide which remote project directory maps to which local project. It does this with a 6-tier matching algorithm, evaluated in order — the first match wins:

| Tier | Strategy | What it compares |
|------|----------|-----------------|
| 1 | **Exact path** | Encoded directory names are identical (same absolute path on both machines). |
| 2 | **Git remote** | The `.meta.json` sidecar carries the project's git remote URL. If any local project shares the same remote, it matches — regardless of where the repo lives on disk. |
| 3 | **Path suffix** | Compares path segments from the right. `/home/alice/repos/myapp` and `/Users/bob/code/repos/myapp` share the suffix `repos/myapp` (2 segments). A minimum of 2 matching segments is required, and the highest-scoring local project wins. |
| 4 | **Home directory** | If the remote project is the other machine's home directory (detected via the `IsHome` sidecar flag or heuristics like `/home/<user>` on Linux, `/Users/<user>` on macOS), it maps to the local home directory project. |
| 5 | **Manual link** | The user can explicitly map remote directories to local paths via `negent link`. |
| 6 | **Unmatched** | No match found — the file is staged but not placed. Reported in `pull` output as "unmatched". |

The `.meta.json` sidecar (generated alongside each project directory during `push`) carries the metadata that makes tiers 2–4 possible:

```json
{
  "absolute_path": "/home/alice/repos/myapp",
  "git_remote": "git@github.com:alice/myapp.git",
  "os": "linux",
  "segments": ["home", "alice", "repos", "myapp"],
  "is_home": false
}
```

In practice, **tier 2 (git remote)** handles the most common cross-machine case — same repo, different checkout paths. Tier 3 (suffix) covers repos without remotes or with mismatched remote URLs.

Legacy Claude config aliases still accepted on read:

- `config` -> `claude-md`
- `custom-code` -> `commands`, `skills`, `agents`, `rules`, `output-styles`
- `memory` -> `agent-memory`, `auto-memory`
- `sessions` -> `sessions`
- `history` -> `history`
- `plugins` -> `plugins`

## GitHub Copilot CLI

Upstream reference: <https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-config-dir-reference>

Negent syncs these Copilot-specific type IDs:

- `config`: `config.json`
- `mcp`: `mcp-config.json`
- `agents`: `agents/`
- `skills`: `skills/`
- `hooks`: `hooks/`
- `sessions`: `session-state/` (non-default)

Default Copilot sync types:

- `config`
- `mcp`
- `agents`
- `skills`
- `hooks`

Negent intentionally does not sync these Copilot files:

- `permissions-config.json`
- `session-store.db`
- `logs/`
- `installed-plugins/`
- `ide/`
