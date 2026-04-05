---
name: conflicts
description: List sync conflicts between local and remote
---

# negent conflicts

List current sync conflicts (non-interactive mode).

```bash
negent conflicts --list
```

Present the list to the user. If conflicts exist, explain the resolution options:

- `negent conflicts --keep-remote` to accept all remote versions
- `negent conflicts --keep-local` to keep all local versions
- `negent conflicts` in a terminal for interactive resolution

Do NOT run interactive conflict resolution, as it requires TUI interaction.
