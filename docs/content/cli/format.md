---

title: "scut format"
description: "Format source files through scut's formatter integrations."
kicker: "CLI Reference"
tags: ["formatting"]
weight: 80
---
`scut format` is the formatter surface used by hooks and direct agent commands. It reads files from arguments or stdin depending on the subcommand.

Markdown formatting preserves leading Hugo front matter verbatim in YAML, TOML, and JSON forms. If scut cannot safely identify a complete leading front matter block, it leaves the document unchanged rather than risk modifying metadata.

The Markdown formatter uses Goldmark v2 with table, strikethrough, task-list, footnote, and definition-list parsing. It preserves the document's existing prose wrapping and Setext heading style; aggregate GFM linkification is not enabled.

## Generated help

{{< clihelp file="scut-format" command="scut format --help" >}}

{{< clihelp file="scut-format-go" command="scut format go --help" >}}

{{< clihelp file="scut-format-markdown" command="scut format markdown --help" >}}
