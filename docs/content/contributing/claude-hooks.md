---

title: "Claude Hook Implementation"
description: "Claude Code hook command wiring, payload types, and event coverage."
kicker: "Contributing"
tags: ["Claude Code", "hooks"]
weight: 30
---

Claude Code invokes hooks as subprocesses. Scut models the stdin and stdout payloads in the public `hooks/claudecode` package and wires command handlers under `scut claude hook`.

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

Polymorphic fields such as `tool_input`, `tool_response`, `form_schema`, and `user_response` use `json.RawMessage`.

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

## Formatter event

`PostToolUse` is the primary behavior-bearing hook. For Claude Code it matches `Write|Edit`, extracts `file_path` from `tool_input`, checks ignore rules, dispatches by extension, and writes formatted content back to the file.

Most other Claude hook commands currently validate that payloads deserialize and return empty or placeholder outputs. That still gives scut a stable command surface for installing complete hook sets.
