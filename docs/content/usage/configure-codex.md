---

title: "Configure Codex"
description: "Install, inspect, and remove scut entries in Codex hooks.json."
kicker: "Usage"
tags: ["Codex", "hooks"]
weight: 40
---

Scut manages Codex command hooks in `hooks.json`. It intentionally writes JSON hook files rather than inline TOML hook tables so each config layer has one hook representation.

## Install hooks

Project scope writes `.codex/hooks.json`:

```bash
scut codex config install
```

User scope writes `~/.codex/hooks.json`:

```bash
scut codex config install --scope=user
```

By default, Codex setup installs the `post-tool-use` formatter hook. Add `--only` when you want a specific subset:

```bash
scut codex config install --only=post-tool-use
scut codex config install --only=pre-tool-use,post-tool-use
```

## Hook shape

The file model mirrors Codex command-hook configuration:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "apply_patch|Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "scut codex hook post-tool-use",
            "statusMessage": "Formatting..."
          }
        ]
      }
    ]
  }
}
```

## Status and uninstall

```bash
scut codex config status --scope=both
scut codex config status --scope=both --json
scut codex config uninstall
```

{{< note type="info" icon="i" >}}
Project-local Codex hooks only load after the project `.codex/` config layer is trusted by Codex.
{{< /note >}}
