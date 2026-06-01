---

title: "Doctor"
description: "Run read-only diagnostics for Claude Code and Codex setup."
kicker: "Usage"
tags: ["diagnostics"]
weight: 70
---

`scut doctor` reports setup findings without changing files. It is the first command to run when hooks do not fire, formatting does not happen, or Codex/Claude Code config appears stale.

## Common checks

Doctor reports findings for:

- whether `scut` is visible on `PATH`
- parse errors captured by scut logging
- missing scut hook entries
- Codex inline hook conflicts
- disabled Codex hooks
- project trust reminders

## Agent selection

```bash
scut doctor
scut doctor --claude
scut doctor --codex
scut doctor --scope=both
```

Use JSON output for automation:

```bash
scut doctor --json
scut doctor --codex --scope=both --json
```

## Severity model

| Severity | Meaning                                                     |
| -------- | ----------------------------------------------------------- |
| `ok`     | Expected setup is present.                                  |
| `info`   | Useful context that does not require action.                |
| `warn`   | Configuration may work but has a known risk or ambiguity.   |
| `error`  | Setup is missing, invalid, or cannot be used as configured. |
