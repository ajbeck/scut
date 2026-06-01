---

title: "Codex Hook Implementation"
description: "Codex command-hook payloads, parity boundaries, and formatter behavior."
kicker: "Contributing"
tags: ["Codex", "hooks"]
weight: 40
---

Codex command hooks run as subprocesses. Scut models payloads in `hooks/codex` and wires handlers under `scut codex hook`.

## Payload design

Common input fields include:

- `session_id`
- `turn_id`
- `cwd`
- `hook_event_name`
- `model`
- `permission_mode`

Tool payloads use `json.RawMessage` because Bash, `apply_patch`, file-edit tools, and MCP tools all use different JSON shapes.

## Formatter extraction

The Codex `PostToolUse` formatter extracts file paths from direct `file_path` fields, patch text in `command`, `patch`, or `input`, and `apply_patch` hunks. It supports add, update, delete, and move hunk headers.

The formatter only runs when it can identify changed files. Invalid or unrelated tool input is ignored rather than blocking the agent.

## Parity boundaries

Codex and Claude Code hook ecosystems are not identical. Scut only exposes Codex events that are currently documented for Codex command hooks. Claude-only events such as `PostToolUseFailure`, worktree hooks, task hooks, and MCP elicitation hooks remain under `scut claude`.
