---

title: "scut init"
description: "Unified setup for supported agent integrations."
kicker: "CLI Reference"
tags: ["setup"]
weight: 10
---

`scut init` installs scut hook configuration for supported agents.

```bash
scut init
scut init --all
scut init --claude
scut init --codex
scut init --all --dry-run --verbose
```

Run `--dry-run` first when you want to inspect the generated agent configuration before writing files.

## Generated help

{{< clihelp file="scut-init" command="scut init --help" >}}
