# negent

[![CI](https://github.com/manmartgarc/negent/actions/workflows/ci.yml/badge.svg)](https://github.com/manmartgarc/negent/actions/workflows/ci.yml)
[![Release](https://github.com/manmartgarc/negent/actions/workflows/release.yml/badge.svg)](https://github.com/manmartgarc/negent/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

`negent` syncs AI coding assistant config and memory files across machines using a git-backed remote store.

## Why use it

- Keep assistant setup consistent across laptop/workstation/server environments.
- Review and version assistant data with standard Git workflows.
- Preview all sync actions before applying them (`diff`, `push --dry-run`, `pull --dry-run`).
- Resolve local/remote conflicts with interactive or non-interactive flows.

## Support matrix

| Area | Status |
| --- | --- |
| Agents | Claude Code |
| Platforms | Linux, macOS |
| Backend | Git |
| Windows | Not supported yet |

## Installation

### Option 1: Build from source

```bash
git clone https://github.com/manmart/negent.git
cd negent
go build -o negent .
```

### Option 2: Go install

```bash
go install github.com/manmart/negent@latest
```

### Option 3: Prebuilt binaries

Download artifacts from [GitHub Releases](https://github.com/manmart/negent/releases) (published for tagged versions).

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

## Configuration

Default path:

```text
~/.config/negent/config.yaml
```

Top-level fields:

- `backend`: currently `git`
- `repo`: remote URL for sync storage
- `machine`: machine label used in sync metadata
- `agents`: per-agent source and sync-type settings

## Sync types and data model

- Claude sync-type taxonomy and exclusions: [`docs/agent-sync-types.md`](docs/agent-sync-types.md)

## Development

```bash
go build -o negent .
go test ./...
go test ./internal/sync/ -run TestPushPull
go test -cover ./...
go vet ./...
```

## Documentation and community

- Launch runbook: [docs/launch-runbook.md](docs/launch-runbook.md)
- Changelog: [CHANGELOG.md](CHANGELOG.md)
- Contributing: [CONTRIBUTING.md](CONTRIBUTING.md)
- Security policy: [SECURITY.md](SECURITY.md)
- Support: [SUPPORT.md](SUPPORT.md)
- Code of conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)

## License

MIT — see [LICENSE](LICENSE).
