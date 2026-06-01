---

title: "Quickstart"
description: "Install scut and wire a supported agent in a few commands."
kicker: "Usage"
tags: ["install", "hooks"]
weight: 10
---

Scut is a CLI companion for LLM agents. It installs hook commands, formats files after agent edits, renders a Claude Code status line, and diagnoses whether local agent configuration is ready.

## Install the CLI

Use the release installer for the current platform:

{{< command >}}curl -fsSL https://install-scut.ajbeck.dev | sh{{< /command >}}

By default the installer places `scut` in `~/.local/bin`. Make sure that directory is on `PATH` before wiring hooks.

## Configure agents

Preview the planned config changes, then apply them:

{{< command >}}scut init --all --dry-run{{< /command >}}

{{< command >}}scut init --all{{< /command >}}

The dry run shows which agent configuration files would be changed. The real run writes scut-owned hook entries into `.claude/settings.json` and `.codex/hooks.json` when those agents are selected.

{{< note type="tip" icon="✓" >}}
Project scope is intentionally conservative. `scut init` auto-detects Claude Code only when `.claude/` exists and Codex only when `.codex/` exists. Use `--all`, `--claude`, or `--codex` when you want to force setup.
{{< /note >}}

## Check the setup

Check the resulting setup in human-readable output, or emit JSON for automation:

{{< command >}}scut doctor{{< /command >}}

{{< command >}}scut doctor --json{{< /command >}}

Doctor checks PATH visibility, missing hook entries, parse errors, Codex inline hook conflicts, disabled Codex hooks, and project trust reminders.

## Use the formatter hook

The most visible behavior is the `PostToolUse` formatter. After Claude Code writes or edits a Go, Markdown, or MDX file, scut formats that file in place and returns an empty JSON response to the agent.

Supported formatter behavior:

| File type     | Formatter                    | Notes                                 |
| ------------- | ---------------------------- | ------------------------------------- |
| `.go`         | Go formatter via `go/format` | Syntax errors are left unchanged.     |
| `.md`, `.mdx` | `goldmark-prettier-markdown` | Preserves prose wrapping style.       |
| other         | none                         | The file is passed through unchanged. |
