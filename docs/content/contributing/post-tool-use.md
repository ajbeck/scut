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

## Codex path extraction

Codex may provide a direct `file_path`, a command string containing an `apply_patch` body, or tool-specific patch text. Scut parses supported patch headers and extracts candidate paths before dispatching formatters.

## Formatter dispatch

| Extension | Formatter          | Behavior                                                             |
| --------- | ------------------ | -------------------------------------------------------------------- |
| `.go`     | Go formatter       | Uses Go parser/formatter APIs and skips syntactically invalid files. |
| `.md`     | Markdown formatter | Uses `goldmark-prettier-markdown`.                                   |
| `.mdx`    | Markdown formatter | Uses the same Markdown formatter.                                    |

Ignore files such as `.prettierignore` and `.scutignore` can prevent formatting.

## Decision control

The formatter is designed to be transparent. On success it writes formatted files and returns an empty hook response. Unsupported files and missing paths are not errors.
