---

title: "scut claude config"
description: "Install, inspect, and remove Claude Code settings entries."
kicker: "CLI Reference"
tags: ["Claude Code", "config"]
weight: 30
---

```bash
scut claude config install
scut claude config install --scope=user
scut claude config install --dry-run
scut claude config status --scope=both --json
scut claude config uninstall
```

Project scope writes `.claude/settings.json`; user scope writes `~/.claude/settings.json`.

Install and uninstall preserve foreign settings and hook groups.

## Generated help

{{< clihelp file="scut-claude-config" command="scut claude config --help" >}}

{{< clihelp file="scut-claude-config-install" command="scut claude config install --help" >}}

{{< clihelp file="scut-claude-config-status" command="scut claude config status --help" >}}

{{< clihelp file="scut-claude-config-uninstall" command="scut claude config uninstall --help" >}}
