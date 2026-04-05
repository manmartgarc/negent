# negent Plugin for Claude Code

A Claude Code plugin that provides slash commands and auto-sync hooks for [negent](https://github.com/manmart/negent).

## Prerequisites

`negent` must be installed and in your PATH. Install via npm:

```bash
npm install -g negent
```

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
