---

title: "Init Command"
description: "Unified agent setup, detection rules, and dry-run output."
kicker: "Contributing"
tags: ["init", "setup"]
weight: 70
---

`scut init` is the user-facing setup command that coordinates Claude Code and Codex config installers.

## Detection rules

In project scope, init auto-detects supported agents from local config directories:

- `.claude/` enables Claude Code setup.
- `.codex/` enables Codex setup.

Users can force targets with `--all`, `--claude`, or `--codex`.

## Scopes

Init defaults to project scope. User scope delegates to the agent-specific config installers with their user config paths.

## Dry run

`--dry-run` reports selected agents and target config paths without writing. `--verbose` includes rendered config output, which is useful for reviewing exactly what install would merge.

```bash
scut init --all --dry-run --verbose
```

## Logging flags

Init can bake logging options into installed hook commands so future hook subprocesses use the desired log level.
