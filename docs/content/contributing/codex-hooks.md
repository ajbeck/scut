---

title: "Codex Hook Implementation"
description: "Codex command-hook payloads, parity boundaries, and formatter behavior."
kicker: "Contributing"
tags: ["Codex", "hooks"]
weight: 40
---

Codex command hooks run as subprocesses. Scut models payloads in `hooks/codex` and wires handlers under `scut codex hook`.

Codex hooks are trust-gated. Project-local hooks only run after the project `.codex/` layer is trusted. The doctor command reports that as an informational reminder rather than trying to mutate trust state.

## Payload design

Common input fields include:

- `session_id`
- `turn_id`
- `cwd`
- `hook_event_name`
- `model`
- `permission_mode`

Tool payloads use `json.RawMessage` because Bash, `apply_patch`, file-edit tools, and MCP tools all use different JSON shapes.

## Event inventory

| Command              | Event               | Matcher input           | Current behavior                          |
| -------------------- | ------------------- | ----------------------- | ----------------------------------------- |
| `session-start`      | `SessionStart`      | `source`                | Returns placeholder additional context.   |
| `subagent-start`     | `SubagentStart`     | `agent_type`            | Returns placeholder subagent context.     |
| `pre-tool-use`       | `PreToolUse`        | `tool_name` and aliases | Decodes payload and returns empty output. |
| `permission-request` | `PermissionRequest` | `tool_name` and aliases | Decodes payload and returns empty output. |
| `post-tool-use`      | `PostToolUse`       | `tool_name` and aliases | Formats changed Go/Markdown/MDX files.    |
| `pre-compact`        | `PreCompact`        | `trigger`               | Decodes payload and returns empty output. |
| `post-compact`       | `PostCompact`       | `trigger`               | Decodes payload and returns empty output. |
| `user-prompt-submit` | `UserPromptSubmit`  | unused                  | Returns placeholder prompt context.       |
| `subagent-stop`      | `SubagentStop`      | `agent_type`            | Decodes payload and returns empty output. |
| `stop`               | `Stop`              | unused                  | Decodes payload and returns empty output. |

All leaf command structs embed hidden trailing positional args for forward compatibility with future Codex invocation arguments.

## Formatter extraction

The Codex `PostToolUse` formatter extracts file paths from direct `file_path` fields, patch text in `command`, `patch`, or `input`, and `apply_patch` hunks. It supports add, update, delete, and move hunk headers.

The formatter only runs when it can identify changed files. Invalid or unrelated tool input is ignored rather than blocking the agent.

Relative patch paths are resolved against `cwd`. Added, updated, and moved files are candidates for formatting; deleted files are skipped.

## Parity boundaries

Codex and Claude Code hook ecosystems are not identical. Scut only exposes Codex events that are currently documented for Codex command hooks. Claude-only events such as `PostToolUseFailure`, worktree hooks, task hooks, and MCP elicitation hooks remain under `scut claude`.

scut intentionally writes Codex hook configuration to `hooks.json`, not inline `[hooks]` TOML tables. This keeps the config writer JSON-only, narrows ownership, and avoids producing mixed hook representations in one Codex layer.
