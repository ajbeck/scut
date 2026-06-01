---

title: "Doctor Command"
description: "Read-only diagnostics, severity output, and JSON shape."
kicker: "Contributing"
tags: ["doctor", "diagnostics"]
weight: 80
---

`scut doctor` is read-only. It inspects environment and config state, reports findings, and exits without mutating user files.

## Findings

Each finding has:

- agent or subsystem
- severity
- summary
- optional detail
- optional remediation guidance

The command supports human output for terminal sessions and JSON output for automated checks.

## Scope inspection

Doctor can inspect project, user, or both scopes. Missing files are not always errors; an absent config file may be an `info` finding when the user did not ask for that agent.

## JSON output

JSON output is designed for tests and future integrations. Keep it stable when adding new diagnostics, and prefer adding fields over changing existing field meanings.
