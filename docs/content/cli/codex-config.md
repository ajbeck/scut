---

title: "scut codex config"
description: "Install, inspect, and remove Codex hooks.json entries."
kicker: "CLI Reference"
tags: ["Codex", "config"]
weight: 40
---

```bash
scut codex config install
scut codex config install --scope=user
scut codex config status --scope=both --json
scut codex config uninstall
```

Project scope writes `.codex/hooks.json`; user scope writes `~/.codex/hooks.json`.

Scut writes `hooks.json` rather than inline TOML hook tables so ownership stays narrow and predictable.

## Generated help

{{< clihelp file="scut-codex-config" command="scut codex config --help" >}}

{{< clihelp file="scut-codex-config-install" command="scut codex config install --help" >}}

{{< clihelp file="scut-codex-config-status" command="scut codex config status --help" >}}

{{< clihelp file="scut-codex-config-uninstall" command="scut codex config uninstall --help" >}}
