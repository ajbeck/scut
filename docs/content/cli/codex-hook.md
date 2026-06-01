---

title: "scut codex hook"
description: "Codex command-hook subprocess commands."
kicker: "CLI Reference"
tags: ["Codex", "hooks"]
weight: 60
---

Codex invokes these commands as subprocesses and sends command-hook JSON on stdin.

```bash
scut codex hook post-tool-use
scut codex hook pre-tool-use
scut codex hook stop
```

The `post-tool-use` command formats changed Go, Markdown, and MDX files after supported file-edit or patch tools run.
