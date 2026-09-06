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

| Area                   | Purpose                                               |
| ---------------------- | ----------------------------------------------------- |
| `cmd/scut`             | Main released binary entrypoint.                      |
| `cmd/walle`            | Repository-local task runner for development.         |
| `internal/cmd/claude`  | Claude command tree, status line, config, and hooks.  |
| `internal/cmd/codex`   | Codex command tree, config, and hooks.                |
| `internal/cmd/gotools` | Go documentation and module-cache command tree.       |
| `internal/cmd/initcmd` | Unified setup across supported agents.                |
| `internal/cmd/doctor`  | Read-only diagnostics.                                |
| `internal/cmd/mcp`     | MCP utility commands and proxy launchers.             |
| `internal/cmd/update`  | Install-method detection and release binary updates.  |
| `internal/godoc`       | Go source resolution, archives, cache, and rendering. |
| `internal/format`      | Formatter dispatch and ignore handling.               |
| `hooks/claudecode`     | Public Claude Code hook payload types.                |
| `hooks/codex`          | Public Codex hook payload types.                      |

## Build rules

Use Walle for all Go operations. The task runner sets `GOEXPERIMENT=jsonv2` and other required build metadata.

```bash
./walle fmt
./walle vet
./walle test
./walle build
```

Do not call `go test`, `go build`, `go vet`, or `gofmt` directly in this repo.

## Go documentation resolution

`internal/godoc` separates source precedence from remote transport policy.
The generic resolver checks local/workspace source, the standard library, the
read-only Go download cache, and scut-owned archives in order. Its final remote
fetcher owns the complete `GOPROXY` sequence so comma
and pipe fallback cannot be changed accidentally by the generic resolver.

Remote mechanisms sit below that state machine: proxy protocol access handles
HTTP, HTTPS, and file URLs; direct access performs go-import discovery and
in-memory Git cloning. `GONOPROXY`, `GOAUTH`, `GOINSECURE`, and `GOVCS`
are applied at those source, request, transport, and VCS boundaries
respectively. Go environment policy is read directly from process, user
`go/env`, and `GOROOT/go.env` configuration rather than through another Go
subprocess.

Archive integrity is a separate boundary shared by cache reads, proxy downloads,
and exact-version Git clones. It validates complete canonical ZIPs, checks
active module or workspace sums first, and otherwise applies `GOSUMDB` and
`GONOSUMDB`. The scut-owned cache atomically publishes the ZIP, content hash,
and verification provenance as one immutable version entry. Checksum-database
latest-tree checkpoints live in separate scut-owned configuration state so
cache cleanup cannot erase anti-rollback history.
