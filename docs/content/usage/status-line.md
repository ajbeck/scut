---

title: "Claude Status Line"
description: "Render context, model, path, and git state in Claude Code."
kicker: "Usage"
tags: ["Claude Code", "status"]
weight: 50
---

The Claude status line command reads Claude Code's session snapshot from stdin and prints a compact terminal status line.

## Output

The rendered line includes:

- a 20-circle context indicator: 🟣 represents used context and 🟢 unused context
- active model name
- current project path
- git branch
- staged and unstaged change counts
- ahead/behind counts when an upstream branch exists

## Runtime behavior

The command is designed for low latency. It uses `go-git` directly instead of spawning `git`, and it collects git status, branch, ahead/behind, and context rendering concurrently.

Claude Code invokes the command through the `statusLine` setting that `scut claude config install` writes.

Use `scut claude status-line --short` to render a compact 10-circle context indicator instead.

## Claude Code payload

Scut follows Claude Code's [official status-line payload documentation](https://code.claude.com/docs/en/statusline). The command tolerates newly added input fields, so Claude Code can evolve its payload without breaking existing status lines.

For troubleshooting, `scut claude --log-level=debug status-line` records the full incoming payload in the status-line JSONL log. This may contain local paths and session metadata; bare `--log` does not record it.
