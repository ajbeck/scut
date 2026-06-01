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

- context-window usage bar
- active model name
- current project path
- git branch
- staged and unstaged change counts
- ahead/behind counts when an upstream branch exists

## Runtime behavior

The command is designed for low latency. It uses `go-git` directly instead of spawning `git`, and it collects git status, branch, ahead/behind, and context rendering concurrently.

Claude Code invokes the command through the `statusLine` setting that `scut claude config install` writes.

## Context thresholds

| Threshold | Meaning                                         |
| --------- | ----------------------------------------------- |
| 70%       | The context bar shifts to warning color.        |
| 83%       | Claude Code's auto-compaction threshold marker. |

Large-context model detection uses the model ID marker `"[1m]"`, because Claude Code on Bedrock encodes the one-million-token variant in the model ID itself.
