# negent Plugin

A shared plugin package for GitHub Copilot CLI and Claude Code that provides slash commands and auto-sync hooks for [negent](https://github.com/manmart/negent).

## Installation

### GitHub Copilot CLI

From inside Copilot:

```shell
/plugin marketplace add manmartgarc/negent
/plugin install negent@negent
```

From your shell:

```bash
copilot plugin marketplace add manmartgarc/negent
copilot plugin install negent@negent
```

### Claude Code

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

### GitHub Copilot CLI

Load the plugin from the working tree:

```bash
copilot --plugin-dir ./plugin
```

If you test via install instead, reinstall after each change because Copilot caches plugin contents:

```bash
copilot plugin install ./plugin
```

### Claude Code

Load the plugin locally for testing:

```bash
claude --plugin-dir ./plugin
```

Reload after changes:

```
/reload-plugins
```
