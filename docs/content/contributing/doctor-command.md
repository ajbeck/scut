---

title: "Doctor Command"
description: "Read-only diagnostics, severity output, and JSON shape."
kicker: "Contributing"
tags: ["doctor", "diagnostics"]
weight: 80
---

`scut doctor` is read-only. It inspects environment and config state, reports findings, and exits without mutating user files.

Doctor does not call `scut init`, does not repair config, and does not create missing directories. Its job is to explain whether the current setup looks usable and where the user should look next.

## Findings

Each finding has:

- agent or subsystem
- severity
- summary
- optional detail
- optional remediation guidance

The command supports human output for terminal sessions and JSON output for automated checks.

Human output uses four severities: `ok`, `info`, `warn`, and `error`. The command exits non-zero when any finding has `error` severity.

## Checks

| Check           | Behavior                                                                                                                  |
| --------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `scut-path`     | Verifies `scut` is discoverable on `PATH`, because generated hook commands use bare `scut`.                               |
| Claude settings | Looks for project/user `settings.json`, parses JSON when present, and reports scut-owned hook/status entries.             |
| Codex hooks     | Looks for project/user `hooks.json`, parses JSON when present, and reports scut-owned hook entries.                       |
| Codex TOML      | Warns when inline `[hooks]` exists beside `hooks.json`; errors when hook features are disabled.                           |
| Project trust   | Emits an info reminder when project `.codex/` exists because Codex project hooks require the project layer to be trusted. |

## Scope inspection

Doctor can inspect project, user, or both scopes. Missing files are not always errors; an absent config file may be an `info` finding when the user did not ask for that agent.

## JSON output

JSON output is designed for tests and future integrations. Keep it stable when adding new diagnostics, and prefer adding fields over changing existing field meanings.

Each JSON finding includes the severity, agent or subsystem, scope when applicable, check name, optional path, and message. Use new fields for additional machine-readable detail instead of changing existing meanings.
