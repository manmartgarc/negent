# Contributing to negent

Thanks for contributing.

## Development setup

1. Install Go `1.23+` and Git.
2. Clone the repository.
3. Configure repo-managed git hooks:

```bash
git config core.hooksPath .githooks
```

4. Build and run tests:

```bash
go build -o negent .
go test ./...
go vet ./...
```

## Commit messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/). Commit messages drive automated version bumping and changelog generation via [release-please](https://github.com/googleapis/release-please), so getting them right matters.

| Prefix | Version bump | Example |
| --- | --- | --- |
| `fix:` | Patch (`0.0.x`) | `fix(sync): handle empty staging dir` |
| `feat:` | Minor (`0.x.0`) | `feat(cli): add diff command` |
| `feat!:` or `BREAKING CHANGE` footer | Major (`x.0.0`) | `feat!: rename config keys` |
| `chore:`, `docs:`, `ci:`, `test:`, `refactor:` | No release | `docs: update install instructions` |

Scopes are optional but encouraged: `cli`, `plugin`, `sync`, `agent`, `config`, `ci`.

## Making changes

1. Create a branch for your work.
2. Keep changes focused and include tests when behavior changes.
3. Update docs when user-visible behavior changes.
4. Run the standard checks before opening a pull request:

```bash
go test ./...
go vet ./...
```

The pre-commit hook updates and stages `README.md` command table entries automatically.

## Pull requests

Please include:

- A clear summary of what changed and why
- Any behavior or UX impact
- Notes about migration or compatibility impact (if any)

Do **not** manually edit `CHANGELOG.md` — release-please generates it from commit messages.

## Releases

Releases are fully automated via [release-please](https://github.com/googleapis/release-please):

1. Merge conventional commits to `main`.
2. release-please opens (or updates) a **Release PR** that bumps the version in both the CLI and plugin, and updates `CHANGELOG.md`.
3. When you merge the Release PR, release-please creates a git tag (`vX.Y.Z`) and a GitHub Release.
4. The tag triggers the release workflow which builds cross-platform binaries and publishes them.

The CLI binary version and the plugin version (`plugin/.claude-plugin/plugin.json`) are always kept in sync.

## Scope notes

Current implementation is Claude-focused; additions for other agents are welcome as incremental, well-tested contributions.
