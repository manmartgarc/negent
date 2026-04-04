# negent

`negent` syncs AI coding assistant config and memory files across machines using a git-backed remote store.

## Current support

- **Agent support:** Claude Code
- **Platforms:** Linux and macOS (Windows is currently unsupported)
- **Backend:** Git

## Why use it

- Keep assistant setup consistent across laptop/workstation/server environments.
- Review and version your assistant data in a standard Git workflow.
- Preview sync behavior before applying it (`diff`, `push --dry-run`, `pull --dry-run`).

## Installation

### Build from source

```bash
git clone https://github.com/manmart/negent.git
cd negent
go build -o negent .
```

### Install with `go install` (after public release)

```bash
go install github.com/manmart/negent@latest
```

## Quick start

1. Create a Git repository to act as your shared sync store.
2. On machine A, initialize negent:

```bash
negent init --repo git@github.com:<you>/<negent-sync-repo>.git
```

3. Preview and push local data:

```bash
negent push --dry-run
negent push
```

4. On machine B, run init against the same repo, then pull:

```bash
negent init --repo git@github.com:<you>/<negent-sync-repo>.git
negent pull --dry-run
negent pull
```

5. Check ongoing state:

```bash
negent status
negent diff
```

6. If pull detects local/remote conflicts:

```bash
negent conflicts --list
negent conflicts
```

## Configuration

Default config path:

```text
~/.config/negent/config.yaml
```

Top-level config fields:

- `backend`: currently `git`
- `repo`: remote URL for sync storage
- `machine`: machine label used in sync metadata
- `agents`: per-agent source/sync settings

## Sync types and data model

Claude sync-type mapping and exclusions are documented in:

- [`docs/agent-sync-types.md`](docs/agent-sync-types.md)

## Development

```bash
go build -o negent .
go test ./...
go test ./internal/sync/ -run TestPushPull
go test -cover ./...
go vet ./...
```

## Project layout

- `cmd/`: Cobra CLI commands
- `internal/agent/`: assistant-specific file collection and placement
- `internal/backend/`: backend abstraction and Git backend
- `internal/sync/`: orchestrator, planning, conflict detection, merge logic
- `internal/config/`: YAML config loading/saving

## License

MIT — see [LICENSE](LICENSE).
