---
name: pull
description: Pull remote AI assistant configs to this machine
---

# negent pull

Pull remote assistant configuration changes to this machine.

## Steps

1. Preview what will change locally:

```bash
negent diff
```

2. If there are remote changes to pull, ask the user if they want to proceed.

3. If confirmed, execute the pull:

```bash
negent pull
```

4. Report the result. If conflicts are detected, inform the user and suggest running `/negent:conflicts`.
