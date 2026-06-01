---

title: "Config Commands"
description: "How Claude and Codex config installers preserve ownership and foreign settings."
kicker: "Contributing"
tags: ["config", "ownership"]
weight: 60
---

Scut config commands merge owned hook entries into agent config files. The core rule is narrow ownership: scut may add, update, or remove entries that it owns, but it must preserve unrelated user configuration.

## Claude settings model

Claude Code config commands manipulate `settings.json`:

- project scope: `.claude/settings.json`
- user scope: `~/.claude/settings.json`

The model preserves foreign top-level keys and non-scut hook groups. A foreign `statusLine` is protected by `ErrForeignStatusLine`.

Claude uses typed fields for the pieces scut owns and an inline fallback map for unknown top-level keys. That gives us type-safe writes for `hooks` and `statusLine` while preserving future Claude Code settings that scut does not understand.

Default Claude install writes:

- every registered `scut claude hook <slug>` entry
- `statusLine.command = "scut claude status-line"`
- optional baked logging flags between `scut claude` and the leaf command

`--only` may target any hook slug or the literal `status-line`.

## Codex hooks model

Codex config commands manipulate `hooks.json`:

- project scope: `.codex/hooks.json`
- user scope: `~/.codex/hooks.json`

Foreign top-level keys, matcher fields, and command fields round-trip through inline JSON maps so scut does not erase unsupported future fields.

Codex can load hooks from `hooks.json` or inline TOML tables. scut writes `hooks.json` only. That avoids TOML rewrites, keeps ownership narrow, and follows Codex's guidance to avoid mixing hook representations in one config layer.

Default Codex install writes only `post-tool-use`, because it is the Codex hook with real formatter behavior. Explicit `--only` values can install any known Codex hook event.

## Operations

| Operation   | Behavior                                                                                                                          |
| ----------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `install`   | Reads existing config or starts from an empty model, validates `--only`, merges scut-owned groups, and writes deterministic JSON. |
| `uninstall` | Removes only scut-owned hook groups and status-line entries. Missing files are treated as "nothing to remove."                    |
| `status`    | Reports scut-owned entries in project, user, or both scopes. `--json` emits structured output for tests and automation.           |

All writes append a trailing newline and use deterministic JSON formatting so repeated installs are idempotent and diffs stay stable.

## Registry pattern

Both agents use a local registry of installable entries. The registry maps a slug such as `post-tool-use` to an event name, matcher, command, and optional status message. `--only` filters through this registry.

Registry invariants:

- slug names match hook leaf command names
- event names match the agent config key
- matcher values are owned by the registry, not user-provided CLI flags
- status messages are optional and only emitted where the agent config supports them

For Claude, the registry is expected to cover all modeled Claude hook commands plus the separate `status-line` item. For Codex, the registry is limited to events documented by Codex.

## Ownership

Ownership is command-based. scut may update or remove entries whose command invokes scut's own hook surface, but it must preserve foreign entries even when they live under the same event.

Examples of owned command shapes:

```text
scut claude hook post-tool-use
scut claude --log hook post-tool-use
scut claude --log-level=debug hook post-tool-use
scut codex hook post-tool-use
scut codex --log hook post-tool-use
scut codex --log-level=debug hook post-tool-use
```

Mixed groups are treated conservatively. If a hook group contains any foreign command, the group is not considered wholly scut-owned for removal.

## Scope resolution

| Agent       | Project path            | User path                 |
| ----------- | ----------------------- | ------------------------- |
| Claude Code | `.claude/settings.json` | `~/.claude/settings.json` |
| Codex       | `.codex/hooks.json`     | `~/.codex/hooks.json`     |

Project paths are resolved relative to `os.Getwd()` at command time. The config commands do not walk up to a git root. User paths are resolved with `os.UserHomeDir()`.

`install` creates parent directories as needed. `uninstall` and `status` are read-first operations and do not create missing directories.

## Error behavior

| Condition                       | Behavior                                                                        |
| ------------------------------- | ------------------------------------------------------------------------------- |
| Missing file during `install`   | Treat as empty config and create the file.                                      |
| Missing file during `uninstall` | Exit successfully after reporting that there is nothing to remove.              |
| Missing file during `status`    | Report the scope as not existing with no entries.                               |
| Invalid JSON                    | Return a wrapped parse error with the path.                                     |
| Unknown `--only` token          | Return an error wrapping `ErrUnknownOnlyToken` and include the valid token set. |
| Claude foreign `statusLine`     | Return an error wrapping `ErrForeignStatusLine` before writing anything.        |
| Invalid enum flag               | Let Kong reject the command at parse time.                                      |

## Status output

`status --json` emits structured scope results that tests can unmarshal. Human output stays compact for terminal use.
