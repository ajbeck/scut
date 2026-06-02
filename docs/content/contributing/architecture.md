---

title: "Architecture"
description: "How the scut CLI is organized and where new code belongs."
kicker: "Contributing"
tags: ["architecture", "Kong"]
weight: 10
---

Scut is a Go module at `github.com/ajbeck/scut`. All packages live under `internal/` by default; public packages are exposed only when another tool needs to import typed contracts, such as hook payload definitions.

## CLI framework

Scut uses `github.com/alecthomas/kong` for struct-based CLI parsing. Commands are grouped under the root CLI struct and run through `ctx.Run(...)` with dependencies bound at parse time.

The entrypoint follows this shape:

```go
parser := kong.Must(&cli)
ctx, err := parser.Parse(os.Args[1:])
if err != nil {
    logging.LogParseError(os.Args, err)
    parser.FatalIfErrorf(err)
}
err = ctx.Run(bindings...)
```

## Package layout

| Area                   | Purpose                                              |
| ---------------------- | ---------------------------------------------------- |
| `cmd/scut`             | Main binary entrypoint.                              |
| `internal/cmd/claude`  | Claude command tree, status line, config, and hooks. |
| `internal/cmd/codex`   | Codex command tree, config, and hooks.               |
| `internal/cmd/initcmd` | Unified setup across supported agents.               |
| `internal/cmd/doctor`  | Read-only diagnostics.                               |
| `internal/cmd/update`  | Install-method detection and release binary updates. |
| `internal/format`      | Formatter dispatch and ignore handling.              |
| `hooks/claudecode`     | Public Claude Code hook payload types.               |
| `hooks/codex`          | Public Codex hook payload types.                     |

## Build rules

Use Mage for all Go operations. The Magefiles set `GOEXPERIMENT=jsonv2` and other required build metadata.

```bash
mage fmt
mage vet
mage test
mage build
```

Do not call `go test`, `go build`, `go vet`, or `gofmt` directly in this repo.
