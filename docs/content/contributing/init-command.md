---

title: "Init Command"
description: "Unified agent setup, detection rules, and dry-run output."
kicker: "Contributing"
tags: ["init", "setup"]
weight: 70
---

`scut init` is the user-facing setup command that coordinates Claude Code and Codex config installers.

Init does not maintain a separate config writer. It delegates to the same Claude and Codex install functions used by the agent-specific command groups, so merge behavior, ownership rules, and dry-run output stay aligned.

Init never passes `--only`, so each installer's default set applies: the `post-tool-use` formatter hook for both agents, plus the status line for Claude Code. Hooks without real behavior are opt-in through `scut claude config install --only=...` or the Codex equivalent.

## Detection rules

In project scope, init auto-detects supported agents from local config directories:

- `.claude/` enables Claude Code setup.
- `.codex/` enables Codex setup.

Users can force targets with `--all`, `--claude`, or `--codex`.

In user scope, detection checks user config directories and may fall back to PATH lookup for the agent binary. Explicit target flags bypass detection gating.

## Scopes

Init defaults to project scope. User scope delegates to the agent-specific config installers with their user config paths.

Selected targets are preflighted before any files are written. That prevents a partial install where one agent's config is changed and another selected target fails validation before writing.

## Dry run

`--dry-run` reports selected agents and target config paths without writing. `--verbose` includes rendered config output, which is useful for reviewing exactly what install would merge.

```bash
scut init --all --dry-run --verbose
```

## Logging flags

Init can bake logging options into installed hook commands so future hook subprocesses use the desired log level.

`--bake-log-level=LEVEL` implies logging and emits only `--log-level=LEVEL` in generated commands; the extra bare `--log` flag would be redundant. `--bake-log` alone emits `--log`.

## Output behavior

Normal runs print one result line per installed agent and a suggested doctor follow-up:

```bash
scut doctor --scope=project
```

If no supported agents are detected in default mode, init reports each skipped agent and exits successfully without writing files. That keeps `scut init` safe to run in repositories that do not use scut-supported agents yet.
