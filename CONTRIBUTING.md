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

The repository is currently on a pre-1.0 release line, and `release-please` is configured to use conservative pre-1.0 bumps:

| Prefix | Current bump behavior (`0.x`) | Example |
| --- | --- | --- |
| `fix:` | Patch (`0.0.x`) | `fix(sync): handle empty staging dir` |
| `feat:` | Patch (`0.0.x`) | `feat(cli): add diff command` |
| `feat!:` or `BREAKING CHANGE` footer | Minor (`0.x.0`) | `feat!: rename config keys` |
| `chore:`, `docs:`, `ci:`, `test:`, `refactor:` | No release | `docs: update install instructions` |

Once the project moves to `1.0.0+`, standard semantic versioning expectations apply again: `fix` bumps patch, `feat` bumps minor, and breaking changes bump major.

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

### End-to-end test coverage

`go test ./...` also runs the `e2e/` package. Those tests cover full CLI workflows such as `init`, `push`, `pull`, dry-run/status/diff previews, and conflict resolution against a real bare Git remote.

The harness gives each test isolated fake machines by creating separate `HOME`, `XDG_CONFIG_HOME`, and `XDG_DATA_HOME` trees (plus `COPILOT_HOME` when needed), then seeding agent files inside those directories. That lets the suite verify cross-machine sync behavior, staging repo state, and on-disk results without touching a contributor's real dotfiles.

The suite builds the `negent` binary once and shells out to it instead of calling Cobra commands in-process. This keeps the tests aligned with the real executable path that users and CI run, including flag parsing, environment handling, process setup, and stdout/stderr behavior.

## Releases

Releases are automated via [release-please](https://github.com/googleapis/release-please) — do **not** manually edit `CHANGELOG.md`.
While the project remains below `1.0.0`, release automation is configured so `feat:` commits produce patch releases and breaking changes produce minor releases.
