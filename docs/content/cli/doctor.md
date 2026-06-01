---

title: "scut doctor"
description: "Read-only setup diagnostics."
kicker: "CLI Reference"
tags: ["diagnostics"]
weight: 20
---

`scut doctor` checks whether supported agents can find and run scut hooks.

```bash
scut doctor
scut doctor --claude
scut doctor --codex
scut doctor --scope=both
scut doctor --json
```

The command reports `ok`, `info`, `warn`, and `error` findings. JSON output is intended for scripts and future integrations.

## Generated help

{{< clihelp file="scut-doctor" command="scut doctor --help" >}}
