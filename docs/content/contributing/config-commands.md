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

## Codex hooks model

Codex config commands manipulate `hooks.json`:

- project scope: `.codex/hooks.json`
- user scope: `~/.codex/hooks.json`

Foreign top-level keys, matcher fields, and command fields round-trip through inline JSON maps so scut does not erase unsupported future fields.

## Registry pattern

Both agents use a local registry of installable entries. The registry maps a slug such as `post-tool-use` to an event name, matcher, command, and optional status message. `--only` filters through this registry.

## Status output

`status --json` emits structured scope results that tests can unmarshal. Human output stays compact for terminal use.
