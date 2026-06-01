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
