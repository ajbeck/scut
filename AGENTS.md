# scut

CLI tool to be used by LLM agents via hooks, rules and instructions. Provides a consistent interface for agent authors to interact with tools, the environment, and each other.

## Development Rules and Process

- Always use conventional commit syntax, for details see https://www.conventionalcommits.org/en/v1.0.0/. This is required for clean commit history and automated changelog generation.
- The allowed conventional commit types are `feat`, `patch`, `docs`, `refactor`, `test`, and `chore`. Never use other types.
- Never push a branch to a remote without explicit approval from a user.
- Never add a new direct dependency without explicit approval from a user.
- Always use the current latest version of a dependency, unless you have explicit approval from a user.

## Architecture

- **CLI framework**: [github.com/alecthomas/kong](https://github.com/alecthomas/kong) — struct-based CLI parsing with dependency injection via `ctx.Run(binds...)`.
- **Package layout**: All packages go under `internal/` by default. Only expose a public package when explicitly directed.
- **Module**: `github.com/ajbeck/scut`

## Go 1.26

Target version is **Go 1.26**. Use these features where appropriate:

- `new(expr)` — pointer to a computed value without a temp variable.
- `errors.AsType[T]()` — generic, type-safe alternative to `errors.As`.
- Self-referential generic constraints — e.g., `type Adder[A Adder[A]] interface`.
- `T.ArtifactDir()` — test artifact directory (replaces ad-hoc temp dirs in tests).
- `go fix ./...` — modernizer pass; run it to adopt latest idioms.
- Green Tea GC is now default (no action needed).
- `reflect.Type.Fields()`, `.Methods()`, `.Ins()`, `.Outs()` return iterators.
- `log/slog.NewMultiHandler()` — fan-out to multiple slog handlers.
- `io.ReadAll()` is faster and allocates less.
- `bytes.Buffer.Peek(n)` — read without advancing.

## Mage Targets

**DO NOT run `go test`, `go build`, `go vet`, or `gofmt` directly.** Always use the corresponding Mage target. Magefiles set `GOEXPERIMENT=jsonv2` and other required environment configuration automatically. Running Go toolchain commands directly will produce incorrect results or miss build tags.

| Command            | What it does                                             |
| ------------------ | -------------------------------------------------------- |
| `mage test`        | Run all tests with race detector (`go test -race ./...`) |
| `mage build`       | Compile binary into `bin/` with version ldflags          |
| `mage vet`         | Run `go vet` across all packages                         |
| `mage fmt`         | Run `gofmt -w` on all source files                       |
| `mage localDeploy` | Build and copy binary to local bin directory             |
| `mage docs`        | Build the Hugo documentation site into `public/`         |

This applies to all contexts: manual terminal use, CI, agent tool calls, and hook scripts. If you need to run tests for a single package, use `mage test` — do not construct a `go test` invocation yourself.

## JSON v2

We use `encoding/json/v2` (the new JSON package). This requires:

- **Build tag**: All `.go` files that import `encoding/json/v2` or `encoding/json` (v1 shimmed by v2) must include `//go:build goexperiment.jsonv2` at the top.
- **GOEXPERIMENT**: Set `GOEXPERIMENT=jsonv2` when building/testing (Magefiles should set this).
- Import `encoding/json/v2` for the new API. The v1 `encoding/json` package still works but its behavior is altered by the experiment flag.

## Documentation

Documentation lives in `docs/` as a Hugo site. Markdown files under `docs/content/` are the content source of truth, custom templates live under `docs/layouts/`, and CSS/JS assets live under `docs/assets/`. Build the site with `mage docs`; generated output goes to `public/` and is not committed.

| Document                                                                            | Covers                                                                             |
| ----------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| [usage/installation.md](docs/content/usage/installation.md)                         | Install script, release assets, checksums, and source installs                     |
| [usage/quickstart.md](docs/content/usage/quickstart.md)                             | User-facing setup flow for installing scut, wiring agents, and running diagnostics |
| [usage/configure-claude-code.md](docs/content/usage/configure-claude-code.md)       | Claude Code config install/status/uninstall behavior                               |
| [usage/configure-codex.md](docs/content/usage/configure-codex.md)                   | Codex hooks.json install/status/uninstall behavior                                 |
| [usage/status-line.md](docs/content/usage/status-line.md)                           | Claude Code status line behavior and displayed fields                              |
| [usage/logging.md](docs/content/usage/logging.md)                                   | Structured JSONL logging and cleanup behavior                                      |
| [usage/doctor.md](docs/content/usage/doctor.md)                                     | Read-only setup diagnostics and severity model                                     |
| [contributing/architecture.md](docs/content/contributing/architecture.md)           | Package layout, Kong setup, and build rules                                        |
| [contributing/claude-hooks.md](docs/content/contributing/claude-hooks.md)           | Claude Code hook command implementation and payload types                          |
| [contributing/codex-hooks.md](docs/content/contributing/codex-hooks.md)             | Codex hook command implementation and parity boundaries                            |
| [contributing/post-tool-use.md](docs/content/contributing/post-tool-use.md)         | PostToolUse path extraction and formatter dispatch                                 |
| [contributing/config-commands.md](docs/content/contributing/config-commands.md)     | Config file ownership, registries, and status output                               |
| [contributing/init-command.md](docs/content/contributing/init-command.md)           | Unified agent setup and dry-run behavior                                           |
| [contributing/doctor-command.md](docs/content/contributing/doctor-command.md)       | Doctor command internals and JSON output                                           |
| [contributing/release-workflows.md](docs/content/contributing/release-workflows.md) | Pull request, build, release, Homebrew tap, and Pages workflows                    |

### Commit-time documentation check

**Before every commit**, check whether any changed files are covered by a document in the index above. Matching rules:

- `cmd/scut/main.go` or `internal/cmd/**` changes → review `docs/content/contributing/architecture.md`
- `hooks/claudecode/**` or `internal/cmd/claude/hook/**` changes → review `docs/content/contributing/claude-hooks.md`
- `hooks/codex/**` or `internal/cmd/codex/hook/**` changes → review `docs/content/contributing/codex-hooks.md`
- `internal/cmd/claude/hook/posttooluse.go` or `internal/cmd/codex/hook/posttooluse.go` or `PostToolUseInput`/`PostToolUseOutput` changes → review `docs/content/contributing/post-tool-use.md`
- `internal/cmd/claude/statusline.go` or `hooks/claudecode/statusline.go` changes → review `docs/content/usage/status-line.md`
- `internal/logging/**` or `internal/cmd/logging/**` or `--log`/`--log-level` flag changes → review `docs/content/usage/logging.md`
- `internal/cmd/claude/config/**` changes → review `docs/content/usage/configure-claude-code.md` and `docs/content/contributing/config-commands.md`
- `internal/cmd/codex/config/**` changes → review `docs/content/usage/configure-codex.md` and `docs/content/contributing/config-commands.md`
- `internal/cmd/initcmd/**` changes → review `docs/content/usage/quickstart.md` and `docs/content/contributing/init-command.md`
- `internal/cmd/doctor/**` changes → review `docs/content/usage/doctor.md` and `docs/content/contributing/doctor-command.md`
- `docs/layouts/**` or `docs/assets/**` changes → run `mage docs` and inspect the site in a browser
- Any new docs content page → add it to the documentation index above when it describes tracked behavior

If a matching document exists and the commit changes behavior it describes (new bindings, new command groups, changed struct tags, altered command tree layout), update the Markdown docs to reflect the current state **in the same commit**.

@RTK.md
