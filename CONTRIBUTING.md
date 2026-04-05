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

## Releases

Releases are automated via [release-please](https://github.com/googleapis/release-please) — do **not** manually edit `CHANGELOG.md`.
