---

title: "scut logging"
description: "Clean and inspect scut JSONL logs."
kicker: "CLI Reference"
tags: ["logs"]
weight: 70
---

`scut logging` manages records under `~/.scut/logging`.

```bash
scut logging clean --days 14
scut logging clean --all
```

Use cleanup when parse-error or hook logs are no longer useful. The command only targets scut JSONL log files.

## Generated help

{{< clihelp file="scut-logging" command="scut logging --help" >}}

{{< clihelp file="scut-logging-clean" command="scut logging clean --help" >}}
