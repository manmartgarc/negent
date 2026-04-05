---
name: push
description: Push local AI assistant configs to the remote sync store
---

# negent push

Push local assistant configuration changes to the remote git store.

## Steps

1. Preview what will be pushed:

```bash
negent diff
```

2. If there are local changes to push, ask the user if they want to proceed.

3. If confirmed, execute the push:

```bash
negent push
```

4. Report the result. If push fails, show the error and suggest running `/negent:status` for more info.
