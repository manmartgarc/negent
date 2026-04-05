# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
go build -o negent .          # Build binary
go test ./...                  # Run all tests
go test ./internal/sync/       # Run tests for a single package
go test ./internal/sync/ -run TestPushPull  # Run a single test
go test -cover ./...           # Run tests with coverage
go vet ./...                   # Lint

# Run GitHub Actions locally (requires nektos/act as a gh CLI plugin)
gh act -l                      # List available workflows
gh act                         # Run CI workflow locally
gh act --job build-test-vet    # Run a specific job
```

No Makefile — standard `go` toolchain only.

## Architecture

negent is a CLI tool that synchronizes AI coding assistant dotfiles (Claude Code, Copilot, etc.) across machines using a git-backed remote store. It has three core layers:

### Agent Layer (`internal/agent/`)
Abstracts per-assistant file collection and placement. The `Agent` interface defines `Name`, `SourceDir`, `SupportedSyncTypes`, `DefaultSyncTypes`, `NormalizeSyncTypes`, `Collect`, `Place`, `Diff`, and `SyncTypeForPath`. Each agent knows which files to sync (organized by `SyncType`) and how to map project directories across machines.

`registry.go` provides `KnownAgents()` and `DetectAgents()` — a built-in list of agents negent supports (claude, codex, copilot, kiro) and auto-detection based on whether their source directories exist.

The optional `StagingMapper` interface lets agents rewrite `StagingPath` fields during push/diff to target existing cross-machine project directories rather than creating duplicates.

The Claude implementation (`internal/agent/claude/`) handles `~/.claude` and defines fine-grained sync types: `claude-md`, `rules`, `commands`, `skills`, `agents`, `output-styles`, `agent-memory`, `auto-memory`, `sessions`, `history`, `keybindings`. Legacy category names (`config`, `custom-code`, `memory`, etc.) are still accepted as aliases. It uses a 6-tier project matching strategy during `Place`: exact path match → git remote match → path suffix match → home dir match → manual link from config → unmatched.

`docs/agent-sync-types.md` is the repo-level reference for per-agent sync-type mappings and upstream documentation links. Add new agent source-of-truth links there when implementing more agents.

Sidecar `.meta.json` files are generated alongside project directories during `Collect` to carry the original absolute path, path segments, git remote URL, OS, and `IsHome` flag — enabling cross-machine project resolution.

### Backend Layer (`internal/backend/`)
Abstracts remote storage. The `Backend` interface defines `Init`, `Fetch`, `Push`, `Pull`, `Status`, and `StagingDir` operations. The staging directory (`~/.local/share/negent/repo`) is a local git clone that acts as the merge point between agents and the remote.

The git implementation (`internal/backend/git/`) shells out to the `git` CLI.

### Orchestrator (`internal/sync/`)
Coordinates multi-agent sync with conflict detection. **Push** collects files from all agents into the staging dir and pushes to the backend. **Pull** fetches remote changes, snapshots the pre-pull staging state as "base", then for each non-project file compares local vs base to detect unsaved user edits — skipping conflicting files rather than overwriting them.

`history.go` handles JSONL merge/dedup for history files (keyed on timestamp+sessionId).

### Config (`internal/config/`)
YAML config at `~/.config/negent/config.yaml`. Stores backend type, repo URL, machine name, and per-agent settings (source dir, sync types, manual project links).

### CLI (`cmd/`)
Cobra commands: `init` (interactive setup via charmbracelet/huh), `add`, `push`, `pull`, `status`, `link`, `conflicts`.

The CLI is interactive by default (e.g., `init` uses TUI prompts, `conflicts` opens an interactive resolver) but every command must also be fully usable non-interactively via flags. This allows scripting, CI usage, and piping. For example, `conflicts --list` lists without prompting, `conflicts --keep-remote` resolves all without interaction.

### Plugin (`plugin/`)
Claude Code plugin providing slash commands and auto-sync hooks. Contains `.claude-plugin/plugin.json` manifest, `commands/` markdown files for `/negent:push`, `/negent:pull`, etc., and `hooks/hooks.json` for SessionStart/Stop automation. The plugin's `scripts/sync.sh` auto-installs the negent binary from GitHub Releases if not found locally.

## Key Patterns

- **Interfaces over implementations**: `Agent` and `Backend` are interfaces with concrete implementations in sub-packages. Tests use mock implementations in the same test file.
- **Context threading**: All backend and orchestrator methods take `context.Context`.
- **Error wrapping**: Errors are wrapped with `fmt.Errorf("context: %w", err)`.
- **Path encoding**: Local paths are encoded as dash-separated segments for use as directory names in the staging area (e.g., `/home/user/repos/myapp` → `-home-user-repos-myapp`).
- **Excluded files**: The Claude agent excludes credentials, auth tokens, temp files, and transient directories (cache, telemetry, debug, etc.) — see `excludePatterns` and `excludeDirs` in `claude.go`.
