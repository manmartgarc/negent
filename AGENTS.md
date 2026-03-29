# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
go build -o negent .          # Build binary
go test ./...                  # Run all tests
go test ./internal/sync/       # Run tests for a single package
go test ./internal/sync/ -run TestPushPull  # Run a single test
go vet ./...                   # Lint
```

No Makefile, CI config, or linter config exists yet — standard `go` toolchain only.

## Architecture

negent is a CLI tool that synchronizes AI coding assistant dotfiles (Claude Code, Copilot, etc.) across machines using a git-backed remote store. It has three core layers:

### Agent Layer (`internal/agent/`)
Abstracts per-assistant file collection and placement. The `Agent` interface defines `Collect`, `Place`, and `Diff` operations. Each agent knows which files to sync (organized by `Category`: config, custom-code, memory, sessions, history, plugins) and how to map project directories across machines.

The Claude implementation (`internal/agent/claude/`) handles `~/.claude` and uses a 4-tier project matching strategy during `Place`: exact path match → git remote match → path suffix match → manual link from config.

Sidecar `.meta.json` files are generated alongside project directories during `Collect` to carry the original absolute path, git remote URL, and OS — enabling cross-machine project resolution.

### Backend Layer (`internal/backend/`)
Abstracts remote storage. The `Backend` interface defines `Init`, `Fetch`, `Push`, `Pull`, `Status`, and `StagingDir` operations. The staging directory (`~/.local/share/negent/repo`) is a local git clone that acts as the merge point between agents and the remote.

The git implementation (`internal/backend/git/`) shells out to the `git` CLI.

### Orchestrator (`internal/sync/`)
Coordinates multi-agent sync with conflict detection. **Push** collects files from all agents into the staging dir and pushes to the backend. **Pull** fetches remote changes, snapshots the pre-pull staging state as "base", then for each non-project file compares local vs base to detect unsaved user edits — skipping conflicting files rather than overwriting them.

`history.go` handles JSONL merge/dedup for history files (keyed on timestamp+sessionId).

### Config (`internal/config/`)
YAML config at `~/.config/negent/config.yaml`. Stores backend type, repo URL, machine name, and per-agent settings (source dir, sync categories, manual project links).

### CLI (`cmd/`)
Cobra commands: `init` (interactive setup via charmbracelet/huh), `add`, `push`, `pull`, `status`, `link`.

## Key Patterns

- **Interfaces over implementations**: `Agent` and `Backend` are interfaces with concrete implementations in sub-packages. Tests use mock implementations in the same test file.
- **Context threading**: All backend and orchestrator methods take `context.Context`.
- **Error wrapping**: Errors are wrapped with `fmt.Errorf("context: %w", err)`.
- **Path encoding**: Local paths are encoded as dash-separated segments for use as directory names in the staging area (e.g., `/home/user/repos/myapp` → `-home-user-repos-myapp`).
- **Excluded files**: The Claude agent excludes credentials, auth tokens, temp files, and transient directories (cache, telemetry, debug, etc.) — see `excludedFiles` and `excludedDirs` in `claude.go`.
