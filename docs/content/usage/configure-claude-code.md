---

title: "Configure Claude Code"
description: "Install, inspect, and remove scut entries in Claude Code settings."
kicker: "Usage"
tags: ["Claude Code", "hooks"]
weight: 30
---

Scut manages Claude Code project and user settings by merging scut-owned entries into `settings.json`. It preserves unrelated keys and foreign hook groups.

## Install hooks

Project scope writes `.claude/settings.json`:

```bash
scut claude config install
```

User scope writes `~/.claude/settings.json`:

```bash
scut claude config install --scope=user
```

Preview changes without writing:

```bash
scut claude config install --dry-run
scut claude config install --dry-run --json
```

## Installed entries

Claude setup installs entries for all supported Claude Code hook events plus the status line. The formatter entry is the one most users notice:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write|Edit",
        "hooks": [
          {
            "type": "command",
            "command": "scut claude hook post-tool-use",
            "statusMessage": "Formatting..."
          }
        ]
      }
    ]
  }
}
```

## Status and uninstall

Inspect scut-owned entries:

```bash
scut claude config status
scut claude config status --scope=both --json
```

Remove scut-owned entries while preserving foreign settings:

```bash
scut claude config uninstall
scut claude config uninstall --scope=user
```

{{< note type="warn" icon="!" >}}
If a `statusLine` entry exists and is not scut-owned, install refuses to replace it. Remove or migrate that entry intentionally before re-running install.
{{< /note >}}
