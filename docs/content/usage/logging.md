---

title: "Logging"
description: "Understand scut JSONL logs, parse-error records, and cleanup commands."
kicker: "Usage"
tags: ["logs", "diagnostics"]
weight: 60
---

Scut can write structured JSONL logs for hook and command execution. Logs are intended for agent debugging: they capture what command ran, where it ran, and what failed without requiring users to reproduce a full terminal session.

## File layout

Logs live under `~/.scut/logging/`:

```text
~/.scut/logging/
  20260403_post-tool-use.jsonl
  20260403_status-line.jsonl
  20260403_parse-errors.jsonl
  20260402_post-tool-use.jsonl
```

Filenames use `YYYYMMDD_<command-name>.jsonl`, where the command name is the leaf command path.

## Parse errors

Kong parse failures are logged before command execution exits. The parse-error log captures the full argv and the parser error message, which helps diagnose stale hook commands in settings files.

## Cleanup

Use the logging command to remove old records:

```bash
scut logging clean --older-than 14d
scut logging clean --all
```

{{< note type="warn" icon="!" >}}
`--all` removes every JSONL log file in the logging directory, including rotated files. Use it only when you intentionally want a clean log directory.
{{< /note >}}
