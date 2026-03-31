# PostToolUse Hook

`botctrl hook claude post-tool-use`

## Overview

Fires after a Claude Code tool call succeeds. The hook silently formats files written by Claude — applying `gofmt` for Go source and goldmark-prettier-markdown for Markdown. Formatting happens in-place; the hook never blocks tool results or injects context.

## Input

Deserialized from stdin as `claudecode.PostToolUseInput`.

| Field | Type | Description |
|-------|------|-------------|
| *(base fields)* | `claudecode.Input` | `session_id`, `transcript_path`, `cwd`, `hook_event_name`, `permission_mode`, `agent_id`, `agent_type` |
| `tool_name` | `string` | Name of the tool that was called (e.g., `Bash`, `Edit`, `mcp__github__search_repositories`) |
| `tool_use_id` | `string` | Unique identifier for this tool invocation |
| `tool_input` | `json.RawMessage` | The input that was passed to the tool — shape varies per tool |
| `tool_response` | `json.RawMessage` | The tool's response — shape varies per tool |

### FilePath extraction

`PostToolUseInput.FilePath()` extracts `tool_input.file_path` from the raw JSON. Returns `""` if the field is absent, empty, or not a string. This method lives in the shared `hooks/claudecode` package.

## Output

Written to stdout as `claudecode.PostToolUseOutput`.

| Field | Type | Description |
|-------|------|-------------|
| *(base fields)* | `claudecode.BaseOutput` | `continue`, `stopReason`, `suppressOutput`, `systemMessage` |
| `decision` | `*Decision` | Set to `"block"` to reject the tool result |
| `reason` | `*string` | Explanation sent to Claude when blocking |
| `additionalContext` | `*string` | Text injected into Claude's context |
| `updatedMCPToolOutput` | `json.RawMessage` | Replacement output for MCP tools — Claude sees this instead of the original response |

The current implementation always returns an empty `PostToolUseOutput{}`. Formatting is silent — Claude never knows it happened.

## Architecture

### Dispatch flow

1. Decode `PostToolUseInput` from stdin
2. Extract `file_path` via `in.FilePath()` — bail if empty
3. `fs.Stat(fp)` — bail if file doesn't exist
4. Switch on `filepath.Ext(fp)` to select a formatter — bail if no match
5. `afero.ReadFile(fs, fp)` — bail on error
6. Call formatter — bail if result is `nil` (declined) or unchanged
7. `afero.WriteFile(fs, fp, formatted, info.Mode())` — preserves original file permissions
8. Write empty `PostToolUseOutput{}` to stdout

Every bail path writes an empty output and returns nil. The hook never errors on formatting failures — it silently skips.

### Byte formatters

Formatters are pure functions with signature `func(src []byte) ([]byte, error)`:

| Function | Extensions | Backend | Behavior on bad input |
|----------|------------|---------|----------------------|
| `formatGo` | `.go` | `go/format.Source` | Returns `nil, nil` (syntax error — let the compiler catch it) |
| `formatMarkdown` | `.md`, `.mdx` | goldmark + `goldmark-prettier-markdown` with `ProseWrapPreserve` | Returns `nil, nil` (parse error) |

Returning `nil, nil` means "decline to format" — the command skips the write.

### File I/O via afero.Fs

The command receives `afero.Fs` via Kong dependency injection (see [kong-base-setup.md](kong-base-setup.md)). In production this is `afero.OsFs` (real filesystem). In tests it's `afero.MemMapFs` (in-memory), which allows seeding files and verifying writes without touching disk.

### Adding a new formatter

1. Create `format_<name>.go` with a `func format<Name>(src []byte) ([]byte, error)` function
2. Add test cases in `format_<name>_test.go` — pure bytes in, bytes out
3. Add a `case` to the extension switch in `posttooluse.go`
4. Add a dispatch test case in `posttooluse_test.go`

## Testing

Three layers, each independently testable:

| Layer | File | What it tests | Mocks |
|-------|------|---------------|-------|
| FilePath extraction | `hooks/claudecode/claudecode_test.go` | JSON parsing of `tool_input.file_path` | None — pure JSON |
| Byte formatters | `format_go_test.go`, `format_markdown_test.go` | Formatting correctness, edge cases | None — pure bytes |
| Command dispatch | `posttooluse_test.go` | Extension routing, file I/O, end-to-end | `afero.MemMapFs` |

## Code

- **Command**: `internal/cmd/hook/claude/posttooluse.go`
- **Go formatter**: `internal/cmd/hook/claude/format_go.go`
- **Markdown formatter**: `internal/cmd/hook/claude/format_markdown.go`
- **Types**: `hooks/claudecode/claudecode.go` — `PostToolUseInput`, `PostToolUseOutput`, `FilePath()`
