---

title: "scut claude hook"
description: "Claude Code hook subprocess commands."
kicker: "CLI Reference"
tags: ["Claude Code", "hooks"]
weight: 50
---

Claude Code invokes these commands as subprocesses and sends JSON on stdin.

```bash
scut claude hook post-tool-use
scut claude hook pre-tool-use
scut claude hook post-tool-batch
scut claude hook stop
scut claude hook notification
```

Scut exposes all currently modeled Claude Code hook events. The formatter behavior lives in `post-tool-use`; many other handlers currently validate payloads and return empty or placeholder JSON responses.

## Generated help

{{< clihelp file="scut-claude-hook" command="scut claude hook --help" >}}
