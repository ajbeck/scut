---

title: "scut format"
description: "Format source files through scut's formatter integrations."
kicker: "CLI Reference"
tags: ["formatting"]
weight: 80
---
`scut format` is the formatter surface used by hooks and direct agent commands. With no file arguments it is a stdin-to-stdout filter; input that the formatter declines is passed through unchanged. Named files are formatted atomically in place and do not write their contents to stdout.

Each changed file is written to a sibling temporary file, assigned the original mode, synchronized, and renamed over the target. Symbolic links remain links and their resolved targets are replaced. Atomicity is per file: if a later file fails, earlier successful replacements remain committed.

Use `--check` with one or more named files to perform the same formatting and ignore checks without writing. The command succeeds when every selected file is already formatted and otherwise fails with the changed paths in argument order. Repeated paths are reported once. `--force` includes paths excluded by `.prettierignore` or `.scutignore` in either write or check mode.

Markdown formatting preserves leading Hugo front matter verbatim in YAML, TOML, and JSON forms. If scut cannot safely identify a complete leading front matter block, it leaves the document unchanged rather than risk modifying metadata.

The Markdown formatter uses Goldmark v2 with table, strikethrough, task-list, footnote, and definition-list parsing. It preserves the document's existing prose wrapping and Setext heading style; aggregate GFM linkification is not enabled.

The direct Markdown command accepts these formatting options:

| Option           | Default    | Behavior                                                                                                             |
| ---------------- | ---------- | -------------------------------------------------------------------------------------------------------------------- |
| `--prose-wrap`   | `preserve` | `preserve` retains source prose line breaks, `always` wraps prose to the print width, and `never` joins prose lines. |
| `--print-width`  | `80`       | Sets the positive target width used by prose wrapping and compact tables.                                            |
| `--tab-width`    | `2`        | Sets the positive indentation width used to align list content.                                                      |
| `--single-quote` | disabled   | Uses single quotes instead of double quotes for link and image titles.                                               |

These options apply only to the current direct command invocation. Agent hooks continue to use the stable defaults and do not read persistent formatter configuration.

## Generated help

{{< clihelp file="scut-format" command="scut format --help" >}}

{{< clihelp file="scut-format-go" command="scut format go --help" >}}

{{< clihelp file="scut-format-markdown" command="scut format markdown --help" >}}
