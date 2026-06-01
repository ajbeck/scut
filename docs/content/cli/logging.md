---

title: "scut logging"
description: "Clean and inspect scut JSONL logs."
kicker: "CLI Reference"
tags: ["logs"]
weight: 70
---

`scut logging` manages records under `~/.scut/logging`.

```bash
scut logging clean --older-than 14d
scut logging clean --all
```

Use cleanup when parse-error or hook logs are no longer useful. The command only targets scut JSONL log files.
