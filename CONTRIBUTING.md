# Contributing to negent

Thanks for contributing.

## Development setup

1. Install Go `1.23+` and Git.
2. Clone the repository.
3. Build and run tests:

```bash
go build -o negent .
go test ./...
go vet ./...
```

## Making changes

1. Create a branch for your work.
2. Keep changes focused and include tests when behavior changes.
3. Update docs when user-visible behavior changes.
4. Run the standard checks before opening a pull request:

```bash
go test ./...
go vet ./...
```

## Pull requests

Please include:

- A clear summary of what changed and why
- Any behavior or UX impact
- Notes about migration or compatibility impact (if any)

## Scope notes

Current implementation is Claude-focused; additions for other agents are welcome as incremental, well-tested contributions.
