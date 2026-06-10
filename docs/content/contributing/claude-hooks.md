---

title: "Claude Hook Implementation"
description: "Claude Code hook command wiring, payload types, and event coverage."
kicker: "Contributing"
tags: ["Claude Code", "hooks"]
weight: 30
---

Claude Code invokes hooks as subprocesses. Scut models the stdin and stdout payloads in the public `hooks/claudecode` package and wires command handlers under `scut claude hook`.

## Process contract

Claude Code pipes one JSON payload to stdin and reads JSON from stdout. Exit behavior matters:

| Exit code      | Meaning                                                                |
| -------------- | ---------------------------------------------------------------------- |
| `0`            | Success. Claude Code parses stdout as the hook response.               |
| `2`            | Blocking error. stderr is surfaced to the user as the blocking reason. |
| other non-zero | Non-blocking error. stderr is logged and Claude continues.             |

Hook commands should return typed JSON responses, not prose. Human-facing diagnostics belong on stderr or in structured logs.

## Shared type package

Every Claude hook input embeds `claudecode.Input`, which carries common fields:

- `session_id`
- `transcript_path`
- `cwd`
- `hook_event_name`
- `permission_mode`
- optional object-shaped `effort`
- `agent_id`
- `agent_type`

Polymorphic fields such as `tool_input`, `tool_response`, `requested_schema`, and elicitation `content` use `json.RawMessage`.

## Command structure

Hook commands live in `internal/cmd/claude/hook`. Each command deserializes stdin, performs any event-specific behavior, and writes a JSON response.

```go
func (c *postToolBatchCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error {
    var in cc.PostToolBatchInput
    if err := json.NewDecoder(stdin).Decode(&in); err != nil {
        return fmt.Errorf("decoding PostToolBatch input: %w", err)
    }
    return writeJSON(stdout, cc.PostToolBatchOutput{})
}
```

Each leaf command embeds hidden trailing positional args. This keeps the command surface forward-compatible if Claude Code appends new positional values to hook invocations.

## Event inventory

| Area                     | Commands                                                                             |
| ------------------------ | ------------------------------------------------------------------------------------ |
| Setup/session            | `setup`, `session-start`, `session-end`                                              |
| Instructions and prompts | `instructions-loaded`, `user-prompt-submit`, `user-prompt-expansion`, `message-display` |
| Tool use                 | `pre-tool-use`, `post-tool-use`, `post-tool-use-failure`, `post-tool-batch`          |
| Permissions              | `permission-request`, `permission-denied`                                            |
| Notifications            | `notification`                                                                       |
| Subagents and stop       | `subagent-start`, `subagent-stop`, `stop`, `stop-failure`                            |
| Team/task events         | `task-created`, `task-completed`, `teammate-idle`                                    |
| Config/files/worktrees   | `config-change`, `cwd-changed`, `file-changed`, `worktree-create`, `worktree-remove` |
| Compaction and MCP       | `pre-compact`, `post-compact`, `elicitation`, `elicitation-result`                   |

Decision-capable events use event-specific output shapes rather than a single universal response. Keep the public `hooks/claudecode` structs aligned with Claude Code's documented wire shape.

## Formatter event

`PostToolUse` is the primary behavior-bearing hook. For Claude Code it matches `Write|Edit`, extracts `file_path` from `tool_input`, checks ignore rules, dispatches by extension, and writes formatted content back to the file.

Most other Claude hook commands currently validate that payloads deserialize and return empty or placeholder outputs. They are not installed by default, but the stable command surface lets users opt in to any event with `--only`.

## Implementation maturity

`post-tool-use` is fully implemented and formats supported files. Several lifecycle hooks emit structured logs with useful fields and return minimal allow/context responses. Remaining commands intentionally deserialize input and return valid placeholder or empty outputs until repository-specific policy is added.

When adding real behavior to a placeholder command:

1. Update the public input/output type if the wire contract changes.
2. Keep stderr reserved for hook errors; stdout must remain machine-readable JSON.
3. Add fixture-driven decode/encode tests for the event.
4. Update the config registry if the hook should be installed by default or needs a matcher/status message.
