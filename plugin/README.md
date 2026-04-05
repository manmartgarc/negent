# negent Plugin for Claude Code

A Claude Code plugin that provides slash commands and auto-sync hooks for [negent](https://github.com/manmart/negent).

## Installation

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

## Commands

| Command | Description |
|---------|-------------|
| `/negent:push` | Push local configs to remote |
| `/negent:pull` | Pull remote configs to this machine |
| `/negent:status` | Show sync status |
| `/negent:diff` | Preview sync differences |
| `/negent:conflicts` | List and resolve conflicts |

## Hooks

The plugin automatically syncs on:

- **Session start**: pushes local changes then pulls remote changes
- **Session stop**: pushes final state

## Development

Load the plugin locally for testing:

```bash
claude --plugin-dir ./plugin
```

Reload after changes:

```
/reload-plugins
```
