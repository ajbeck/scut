# Claude Code Hook Commands

`botctrl hook claude <event>` handles Claude Code hook events. Each event type is a separate subcommand that reads a JSON payload from stdin and writes a JSON response to stdout.

## How It Works

Claude Code invokes hooks as subprocesses. The flow is:

1. Claude Code pipes a JSON payload to the hook process's stdin.
2. The hook reads stdin, deserializes into the event-specific input type.
3. The hook writes a JSON response to stdout.
4. Claude Code reads stdout and applies the response (inject context, allow/deny tool calls, block prompts, etc.).

Exit codes control behavior:
- **0** — success, stdout JSON is parsed as the response.
- **2** — blocking error, stderr is shown as the error message.
- **non-zero (not 2)** — non-blocking error, stderr is logged, execution continues.

## Shared Types Package

All input and output types live in the **exported** package `hooks/claudecode`. This is intentionally not under `internal/` — it is a public API so external tools can also model Claude Code hook payloads.

### Type Design

- Every input type embeds `claudecode.Input` which carries the common fields (`session_id`, `transcript_path`, `cwd`, `hook_event_name`, `permission_mode`, `agent_id`, `agent_type`).
- Every output type embeds `claudecode.BaseOutput` which carries universal response fields (`continue`, `stopReason`, `suppressOutput`, `systemMessage`).
- Enum-like values use typed string types with constants (e.g., `EventName`, `PermissionDecision`, `StopError`). This gives compile-time discoverability without sacrificing JSON compatibility.
- Output fields use pointer types for base types (`*string`, `*bool`, `*Decision`) so unset fields are omitted from JSON via `omitempty`. Use `new(expr)` to set them inline.
- Input `bool` fields use `*bool` to distinguish `false` from absent. Input string fields remain values since empty string and absent are functionally equivalent.
- `json.RawMessage` is used for polymorphic fields (`tool_input`, `tool_response`, `form_schema`, `user_response`) that vary by tool or MCP server.

## Command Structure

Commands live in `internal/cmd/hook/claude/`. Each command is an unexported struct with a `Run(stdin io.Reader, stdout io.Writer) error` method. `io.Reader` and `io.Writer` are injected by Kong via `BindTo` (see [kong-base-setup.md](kong-base-setup.md)).

## Event Reference

### Session Lifecycle

| Command | Event | Input Type | Output Type | Decision Control |
|---------|-------|-----------|-------------|-----------------|
| `session-start` | SessionStart | `SessionStartInput` | `SessionStartOutput` | Can inject context via `additionalContext` |
| `session-end` | SessionEnd | `SessionEndInput` | `BaseOutput` | Observability only |

**session-start** fires when a session begins or resumes. The `source` field indicates the trigger (`startup`, `resume`, `clear`, `compact`). The response can inject text into Claude's context via `additionalContext`.

**session-end** fires when a session terminates. The `reason` field indicates why (`clear`, `resume`, `logout`, `prompt_input_exit`, `bypass_permissions_disabled`, `other`). No decision control — observability only.

### Instructions

| Command | Event | Input Type | Output Type | Decision Control |
|---------|-------|-----------|-------------|-----------------|
| `instructions-loaded` | InstructionsLoaded | `InstructionsLoadedInput` | `BaseOutput` | Observability only |

**instructions-loaded** fires when a CLAUDE.md or rules file is loaded. Provides `file_path`, `memory_type` (User/Project/Local/Managed), `load_reason`, and optional glob/trigger/parent paths. Observability only.

### User Input

| Command | Event | Input Type | Output Type | Decision Control |
|---------|-------|-----------|-------------|-----------------|
| `user-prompt-submit` | UserPromptSubmit | `UserPromptSubmitInput` | `UserPromptSubmitOutput` | Can block prompt or inject context |

**user-prompt-submit** fires when the user submits a prompt, before Claude processes it. The response can set `decision` to `"block"` with a `reason` to reject the prompt, or inject `additionalContext`.

### Tool Use

| Command | Event | Input Type | Output Type | Decision Control |
|---------|-------|-----------|-------------|-----------------|
| `pre-tool-use` | PreToolUse | `PreToolUseInput` | `PreToolUseOutput` | Can allow/deny/ask, modify input, inject context |
| `post-tool-use` | PostToolUse | `PostToolUseInput` | `PostToolUseOutput` | Can block result, inject context, modify MCP output |
| `post-tool-use-failure` | PostToolUseFailure | `PostToolUseFailureInput` | `PostToolUseFailureOutput` | Can inject context |

**pre-tool-use** fires before a tool call executes. The response wraps decisions in `hookSpecificOutput` with `permissionDecision` (`allow`, `deny`, `ask`), an optional `permissionDecisionReason`, `updatedInput` to modify the tool input, and `additionalContext`.

**post-tool-use** fires after a tool call succeeds. Can set `decision` to `"block"` to reject the result, inject `additionalContext`, or provide `updatedMCPToolOutput` for MCP tools.

**post-tool-use-failure** fires after a tool call fails. Provides `error` and `is_interrupt`. Can inject `additionalContext`.

### Permissions

| Command | Event | Input Type | Output Type | Decision Control |
|---------|-------|-----------|-------------|-----------------|
| `permission-request` | PermissionRequest | `PermissionRequestInput` | `PermissionRequestOutput` | Can auto-approve/deny, modify input, update permissions |

**permission-request** fires when a permission dialog is about to appear. Input includes `permission_suggestions` — proposed rule changes. The response wraps decisions in `hookSpecificOutput` with a `decision` containing `behavior` (`allow`/`deny`), optional `updatedInput`, `updatedPermissions`, `message`, and `interrupt` flag.

### Notifications

| Command | Event | Input Type | Output Type | Decision Control |
|---------|-------|-----------|-------------|-----------------|
| `notification` | Notification | `NotificationInput` | `NotificationOutput` | Can inject context |

**notification** fires when Claude Code sends a notification. The `notification_type` field categorizes it (`permission_prompt`, `idle_prompt`, `auth_success`, `elicitation_dialog`). Can inject `additionalContext`.

### Subagents

| Command | Event | Input Type | Output Type | Decision Control |
|---------|-------|-----------|-------------|-----------------|
| `subagent-start` | SubagentStart | `SubagentStartInput` | `SubagentStartOutput` | Can inject context |
| `subagent-stop` | SubagentStop | `SubagentStopInput` | `SubagentStopOutput` | Can block stop |

**subagent-start** fires when a subagent is spawned. `agent_id` and `agent_type` are in the base `Input` fields. Can inject `additionalContext`.

**subagent-stop** fires when a subagent finishes. Includes `stop_hook_active`, `agent_transcript_path`, and `last_assistant_message`. Can set `decision` to `"block"` with a `reason` to prevent the subagent from stopping.

### Stop

| Command | Event | Input Type | Output Type | Decision Control |
|---------|-------|-----------|-------------|-----------------|
| `stop` | Stop | `StopInput` | `StopOutput` | Can block stop |
| `stop-failure` | StopFailure | `StopFailureInput` | `BaseOutput` | Observability only |

**stop** fires when Claude finishes responding. Includes `stop_hook_active` and `last_assistant_message`. Can set `decision` to `"block"` with a `reason` to force Claude to continue.

**stop-failure** fires when a turn ends due to an API error. The `error` field categorizes the failure (`rate_limit`, `authentication_failed`, `billing_error`, `invalid_request`, `server_error`, `max_output_tokens`, `unknown`). Output and exit code are ignored — observability only.

### Tasks and Teams

| Command | Event | Input Type | Output Type | Decision Control |
|---------|-------|-----------|-------------|-----------------|
| `task-created` | TaskCreated | `TaskCreatedInput` | `TaskOutput` | Exit 2 blocks creation |
| `task-completed` | TaskCompleted | `TaskCompletedInput` | `TaskOutput` | Exit 2 blocks completion |
| `teammate-idle` | TeammateIdle | `TeammateIdleInput` | `TeammateIdleOutput` | Exit 2 continues teammate |

**task-created** and **task-completed** fire on task lifecycle events in agent teams. Include `task_id`, `task_subject`, `task_description`, `teammate_name`, `team_name`. Exit code 2 blocks the action.

**teammate-idle** fires when a teammate is about to go idle. Exit code 2 forces the teammate to continue.

### Configuration

| Command | Event | Input Type | Output Type | Decision Control |
|---------|-------|-----------|-------------|-----------------|
| `config-change` | ConfigChange | `ConfigChangeInput` | `ConfigChangeOutput` | Can block change |

**config-change** fires when a configuration file changes. The `source` field identifies the layer (`user_settings`, `project_settings`, `local_settings`, `policy_settings`, `skills`). Can set `decision` to `"block"` to prevent the change from applying.

### File System

| Command | Event | Input Type | Output Type | Decision Control |
|---------|-------|-----------|-------------|-----------------|
| `cwd-changed` | CwdChanged | `CwdChangedInput` | `BaseOutput` | Can write to CLAUDE_ENV_FILE |
| `file-changed` | FileChanged | `FileChangedInput` | `BaseOutput` | Can write to CLAUDE_ENV_FILE |

**cwd-changed** fires when the working directory changes. Provides `new_cwd` and `previous_cwd`. Can write to `CLAUDE_ENV_FILE` to reload environment variables.

**file-changed** fires when a watched file changes on disk. Provides `file_path` and `changed_type` (`create`, `modify`, `delete`). Can write to `CLAUDE_ENV_FILE`.

### Worktrees

| Command | Event | Input Type | Output Type | Decision Control |
|---------|-------|-----------|-------------|-----------------|
| `worktree-create` | WorktreeCreate | `WorktreeCreateInput` | `WorktreeCreateOutput` | Replaces default git worktree behavior |
| `worktree-remove` | WorktreeRemove | `WorktreeRemoveInput` | `BaseOutput` | Observability only |

**worktree-create** fires when a worktree is being created. The response provides a `worktreePath` in `hookSpecificOutput` — this replaces the default git worktree creation behavior.

**worktree-remove** fires when a worktree is being removed. Observability only.

### Context Compaction

| Command | Event | Input Type | Output Type | Decision Control |
|---------|-------|-----------|-------------|-----------------|
| `pre-compact` | PreCompact | `PreCompactInput` | `BaseOutput` | Observability only |
| `post-compact` | PostCompact | `PostCompactInput` | `BaseOutput` | Observability only |

**pre-compact** and **post-compact** fire before and after context compaction. The `trigger` field is `manual` or `auto`. Observability only.

### MCP Elicitation

| Command | Event | Input Type | Output Type | Decision Control |
|---------|-------|-----------|-------------|-----------------|
| `elicitation` | Elicitation | `ElicitationInput` | `ElicitationOutput` | Can accept/decline/cancel |
| `elicitation-result` | ElicitationResult | `ElicitationResultInput` | `ElicitationResultOutput` | Can accept/decline/cancel, modify content |

**elicitation** fires when an MCP server requests user input. Includes `mcp_server_name` and `form_schema`. The response wraps decisions in `hookSpecificOutput` with `action` (`accept`, `decline`, `cancel`) and optional `content`.

**elicitation-result** fires after the user responds to an MCP elicitation, before the response is sent to the server. Includes `mcp_server_name` and `user_response`. Can set `action` and provide modified `content`.

## Current Status

All 25 commands are wired up with hello-world implementations that deserialize stdin and return mock output values. The next step is replacing the mock implementations with real hook logic.
