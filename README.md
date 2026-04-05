<p align="center">
    <picture>
        <source media="(prefers-color-scheme: dark)" srcset="https://github.com/manmartgarc/negent/blob/main/docs/dark-logo.svg">
        <source media="(prefers-color-scheme: light)" srcset="https://github.com/manmartgarc/negent/blob/main/docs/light-logo.svg">
        <img alt="negent — Agent-agnostic dotfile sync" width="600" src="https://github.com/manmartgarc/negent/blob/main/docs/light-logo.svg">
    </picture>
</p>

<p align="center">
    <a href="https://github.com/manmartgarc/negent/actions/workflows/ci.yml"><img src="https://github.com/manmartgarc/negent/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
    <a href="https://github.com/manmartgarc/negent/actions/workflows/release-please.yml"><img src="https://github.com/manmartgarc/negent/actions/workflows/release-please.yml/badge.svg" alt="Release"></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
</p>

`negent` syncs AI coding assistant config and memory files across machines using a git-backed remote store.

## Why use it

- Keep assistant setup consistent across laptop/workstation/server environments.
- Review and version assistant data with standard Git workflows.
- Preview all sync actions before applying them (`diff`, `push --dry-run`, `pull --dry-run`).
- Resolve local/remote conflicts with interactive or non-interactive flows.

## Support matrix

| Area | Status |
| --- | --- |
| Agents | Claude Code, GitHub Copilot CLI |
| Platforms | Linux, macOS |
| Backend | Git |
| Windows | Not supported yet |

## Installation

### Option 1: Go install

```bash
go install github.com/manmart/negent@latest
```

### Option 2: Prebuilt binaries

Download artifacts from [GitHub Releases](https://github.com/manmart/negent/releases) (published for tagged versions).

### Option 3: Build from source

```bash
git clone https://github.com/manmart/negent.git
cd negent
go build -o negent .
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

4. On machine B, point to the same repo and pull:

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

## Command reference

| Command | Purpose |
| --- | --- |
| `add <agent>` | Add another agent to an existing negent setup |
| `auto` | Manage automatic sync hooks |
| `clean <agent>` | Delete local agent configuration on this machine |
| `completion` | Generate the autocompletion script for the specified shell |
| `config edit` | Edit negent configuration |
| `conflicts` | List and resolve sync conflicts |
| `diff` | Preview local and remote sync actions |
| `init` | Initialize negent on this machine |
| `link <agent> <remote-project> <local-path>` | Manually link a remote project to a local path |
| `pull` | Pull remote agent configs to this machine |
| `push` | Push local agent configs to the remote |
| `status` | Show sync status for configured agents |

## Claude Code plugin

negent ships as a Claude Code [plugin](https://code.claude.com/docs/en/plugins) with slash commands and auto-sync hooks.

### Install the plugin

**Option A: Marketplace** (recommended)

From inside Claude Code:

```shell
/plugin marketplace add manmartgarc/negent
/plugin install negent@negent
```

From your shell:

```bash
claude plugin marketplace add manmartgarc/negent
claude plugin install negent@negent
```

The plugin auto-installs the `negent` binary from [GitHub Releases](https://github.com/manmartgarc/negent/releases) on first run if it is not already available on your `PATH`.

**Option B: Local development**

```bash
claude --plugin-dir ./plugin
```

### Slash commands

| Command | Description |
| --- | --- |
| `/negent:push` | Push local configs to remote |
| `/negent:pull` | Pull remote configs to this machine |
| `/negent:status` | Show sync status |
| `/negent:diff` | Preview sync differences |
| `/negent:conflicts` | List and resolve conflicts |

### Auto-sync hooks

The plugin automatically syncs on session start (push then pull) and session stop (push).

### Legacy: settings.json hooks

Alternatively, `negent auto enable` installs hooks directly into `~/.claude/settings.json`. The plugin approach above is preferred.

## License

MIT — see [LICENSE](LICENSE).
