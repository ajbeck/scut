---

title: "scut init"
description: "Unified setup for supported agent integrations."
kicker: "CLI Reference"
tags: ["setup"]
weight: 10
---

`scut init` installs scut hook configuration for supported agents.

```bash
scut init
scut init --all
scut init --claude
scut init --codex
scut init --all --dry-run --verbose
```

| Flag                     | Behavior                                                 |
| ------------------------ | -------------------------------------------------------- |
| `--all`                  | Select all supported agents.                             |
| `--claude`               | Select Claude Code.                                      |
| `--codex`                | Select Codex.                                            |
| `--scope=user`           | Write user-level config instead of project-level config. |
| `--dry-run`              | Show planned changes without writing.                    |
| `--verbose`              | Include rendered config in dry-run output.               |
| `--bake-log-level=LEVEL` | Bake a log level into installed hook commands.           |
