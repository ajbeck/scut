# botctrl

CLI tool to be used by LLM agents via hooks, rules and instructions. Provides a consistent interface for agent authors to interact with tools, the environment, and each other.

## Architecture

- **CLI framework**: [github.com/alecthomas/kong](https://github.com/alecthomas/kong) — struct-based CLI parsing with dependency injection via `ctx.Run(binds...)`.
- **Package layout**: All packages go under `internal/` by default. Only expose a public package when explicitly directed.
- **Module**: `github.com/ajbeck/botctrl`

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

## JSON v2

We use `encoding/json/v2` (the new JSON package). This requires:

- **Build tag**: All `.go` files that import `encoding/json/v2` or `encoding/json` (v1 shimmed by v2) must include `//go:build goexperiment.jsonv2` at the top.
- **GOEXPERIMENT**: Set `GOEXPERIMENT=jsonv2` when building/testing (Magefiles should set this).
- Import `encoding/json/v2` for the new API. The v1 `encoding/json` package still works but its behavior is altered by the experiment flag.

## Implementation Documentation

Implementation docs live in `docs/implementation/`. Each document covers a specific subsystem or design decision.

| Document                                                               | Covers                                                                                                                 |
| ---------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| [kong-base-setup.md](docs/implementation/kong-base-setup.md)           | Kong CLI framework setup, BindTo dependency injection, command tree structure, how to add commands and groups          |
| [claude-hook-commands.md](docs/implementation/claude-hook-commands.md) | Claude Code hook subcommands, event types, input/output types, decision control per event, shared types package design |
| [post-tool-use.md](docs/implementation/post-tool-use.md)               | PostToolUse hook deep-dive — input/output fields, decision control, MCP output replacement, code locations             |

### Commit-time documentation check

**Before every commit**, check whether any changed files are covered by a document in the index above. Matching rules:

- `cmd/botctrl/main.go` or `internal/cmd/**` changes → review `kong-base-setup.md`
- `hooks/claudecode/**` or `internal/cmd/hook/claude/**` changes → review `claude-hook-commands.md`
- `internal/cmd/hook/claude/posttooluse.go` or `PostToolUseInput`/`PostToolUseOutput` changes → review `post-tool-use.md`
- Any new `docs/implementation/*.md` file → add it to the index table above

If a matching document exists and the commit changes behavior it describes (new bindings, new command groups, changed struct tags, altered command tree layout), update the document to reflect the current state **in the same commit**.
