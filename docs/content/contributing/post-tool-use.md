---

title: "PostToolUse Formatter"
description: "How scut extracts changed paths and dispatches formatters after tool calls."
kicker: "Contributing"
tags: ["formatter", "hooks"]
weight: 50
---

The `PostToolUse` formatter is shared in spirit across Claude Code and Codex, but each agent provides a different payload shape.

## Claude path extraction

Claude Code `PostToolUse` events include `tool_name`, `tool_input`, and `tool_response`. For `Write` and `Edit`, scut reads `tool_input.file_path` and formats that file if its extension is supported.

`FilePath()` returns an empty string when `file_path` is absent, empty, or non-string. Missing paths are a skip condition, not an error.

## Codex path extraction

Codex may provide a direct `file_path`, a command string containing an `apply_patch` body, or tool-specific patch text. Scut parses supported patch headers and extracts candidate paths before dispatching formatters.

For Codex, `FilePaths()` checks direct `file_path` first, then parses patch text from fields such as `command`, `patch`, or `input`. Relative paths are resolved against `cwd`. Deleted files are not returned because there is nothing left to format.

## Dispatch flow

For each candidate path:

1. `fs.Stat(path)`; skip missing files.
2. Discover the formatting root by walking up from the file directory until `.git`, `.prettierignore`, or `.scutignore` is found.
3. Load root `.prettierignore`, then root `.scutignore`.
4. Skip ignored paths.
5. Select a formatter by file extension.
6. Read bytes through the injected `afero.Fs`.
7. Run the byte formatter.
8. Skip `nil` formatter results and unchanged output.
9. Write formatted bytes back with the original file mode.
10. Return an empty hook response unless at least one file changed.

## Formatter dispatch

| Extension | Formatter          | Behavior                                                             |
| --------- | ------------------ | -------------------------------------------------------------------- |
| `.go`     | Go formatter       | Uses Go parser/formatter APIs and skips syntactically invalid files. |
| `.md`     | Markdown formatter | Uses `goldmark-prettier-markdown`.                                   |
| `.mdx`    | Markdown formatter | Uses the same Markdown formatter.                                    |

Ignore files such as `.prettierignore` and `.scutignore` can prevent formatting.

`.scutignore` is loaded after `.prettierignore`, so it can add scut-specific exclusions or re-include paths with `!` patterns.

## Decision control

The formatter is designed to be transparent. On success it writes formatted files and returns an empty hook response. Unsupported files and missing paths are not errors.

When one or more files are changed, the command returns nested `hookSpecificOutput.additionalContext` with the event name set to `PostToolUse`. The message tells the agent that scut reformatted files on disk, so it should not assume its original tool input is byte-for-byte current.

## Testing layers

| Layer            | Coverage                                                                                           |
| ---------------- | -------------------------------------------------------------------------------------------------- |
| Path extraction  | Pure JSON tests for Claude `file_path` and Codex patch parsing.                                    |
| Byte formatters  | Pure byte-in/byte-out tests for Go and Markdown formatting.                                        |
| Ignore matching  | Root discovery, pattern precedence, directory globs, and negation.                                 |
| Command dispatch | End-to-end hook tests with `afero.MemMapFs`, including ignored files and multi-file Codex patches. |
