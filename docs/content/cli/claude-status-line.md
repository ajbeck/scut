---

title: "scut claude status-line"
description: "Render the Claude Code status bar from session JSON."
kicker: "CLI Reference"
tags: ["Claude Code", "status"]
weight: 55
---

Claude Code runs `scut claude status-line` as a subprocess and sends session JSON on stdin. The command writes styled status text to stdout.

For user-facing setup notes, see [Claude Status Line]({{< relref "/usage/status-line" >}}).

## Generated help

{{< clihelp file="scut-claude-status-line" command="scut claude status-line --help" >}}
