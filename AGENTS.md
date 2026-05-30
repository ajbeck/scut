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

| Command | What it does |
|---------|-------------|
| `mage test` | Run all tests with race detector (`go test -race ./...`) |
| `mage build` | Compile binary into `bin/` with version ldflags |
| `mage vet` | Run `go vet` across all packages |
| `mage fmt` | Run `gofmt -w` on all source files |
| `mage localDeploy` | Build and copy binary to local bin directory |
| `mage docsStandalone` | Regenerate `docs/design-system-standalone.html` by inlining CSS/JS into the source HTML |

This applies to all contexts: manual terminal use, CI, agent tool calls, and hook scripts. If you need to run tests for a single package, use `mage test` — do not construct a `go test` invocation yourself.

## JSON v2

We use `encoding/json/v2` (the new JSON package). This requires:

- **Build tag**: All `.go` files that import `encoding/json/v2` or `encoding/json` (v1 shimmed by v2) must include `//go:build goexperiment.jsonv2` at the top.
- **GOEXPERIMENT**: Set `GOEXPERIMENT=jsonv2` when building/testing (Magefiles should set this).
- Import `encoding/json/v2` for the new API. The v1 `encoding/json` package still works but its behavior is altered by the experiment flag.

## Implementation Documentation

Implementation docs live in `docs/` as standalone HTML files styled by the shared `scut-docs.css` / `scut-docs.js` design system. **The HTML is the single source of truth** — there are no Markdown counterparts. Edit the HTML directly when behaviour changes; preserve the existing structure (`<section id>` blocks, `table.fields`, `.code-frame` code samples, `.callout` callouts, `<ol class="steps">` procedures, syntax-token spans).

| Document                                                       | Covers                                                                                                                 |
| -------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| [kong-base-setup.html](docs/kong-base-setup.html)             | Kong CLI framework setup, BindTo dependency injection, command tree structure, how to add commands and groups          |
| [claude-hook-commands.html](docs/claude-hook-commands.html)   | Claude Code hook subcommands, event types, input/output types, decision control per event, shared types package design |
| [codex-hook-commands.html](docs/codex-hook-commands.html)     | Codex hook subcommands, event types, input/output types, command-hook config shape, Claude parity boundaries           |
| [post-tool-use.html](docs/post-tool-use.html)                 | PostToolUse hook deep-dive — Claude/Codex input extraction, formatting dispatch, decision control, code locations      |
| [status-line.html](docs/status-line.html)                     | Status line command — colour palette, formatting, go-git integration, available input fields                           |
| [logging.html](docs/logging.html)                             | Structured JSONL logging — flags, file layout, rotation, standardized fields, clean command                            |
| [config-command.html](docs/config-command.html)               | `claude config install`/`uninstall`/`status` — settings.json model, merge semantics, ownership rules, registry, scope resolution, error sentinels |
| [codex-config-command.html](docs/codex-config-command.html)   | `codex config install`/`uninstall`/`status` — hooks.json model, default formatter install, ownership rules, scope resolution |
| [init-command.html](docs/init-command.html)                   | `scut init` — unified agent setup, detection rules, explicit agent selection, dry-run output                           |
| [installation.html](docs/installation.html)                   | Install script, release assets, checksum verification, source installs, and release workflow                                                      |
| [release-workflows.html](docs/release-workflows.html)         | Pull request, reusable build, release tagging, GitHub Release publishing, Pages deployment, and Dependabot automation                              |
| [design-system.html](docs/design-system.html)                 | The docs design system itself — page anatomy, primitives (rail, hero, code frames, callouts, steps), colour tokens, theming, voice, and how other projects can adopt the pattern |
| [design-system-standalone.html](docs/design-system-standalone.html) | Single-file shareable edition of `design-system.html` with CSS and JS inlined. Generated — do not hand-edit; run `mage docsStandalone` after changing the source HTML, CSS, or JS |

### Commit-time documentation check

**Before every commit**, check whether any changed files are covered by a document in the index above. Matching rules:

- `cmd/scut/main.go` or `internal/cmd/**` changes → review `kong-base-setup.html`
- `hooks/claudecode/**` or `internal/cmd/claude/hook/**` changes → review `claude-hook-commands.html`
- `hooks/codex/**` or `internal/cmd/codex/hook/**` changes → review `codex-hook-commands.html`
- `internal/cmd/claude/hook/posttooluse.go` or `internal/cmd/codex/hook/posttooluse.go` or `PostToolUseInput`/`PostToolUseOutput` changes → review `post-tool-use.html`
- `internal/cmd/claude/statusline.go` or `hooks/claudecode/statusline.go` changes → review `status-line.html`
- `internal/logging/**` or `internal/cmd/logging/**` or `--log`/`--log-level` flag changes → review `logging.html`
- `internal/cmd/claude/config/**` changes → review `config-command.html`
- `internal/cmd/codex/config/**` changes → review `codex-config-command.html`
- `internal/cmd/initcmd/**` changes → review `init-command.html`
- `docs/scut-docs.css` or `docs/scut-docs.js` changes → review `design-system.html` (class names and conventions documented there), then run `mage docsStandalone` to refresh `design-system-standalone.html`
- `docs/design-system.html` changes → run `mage docsStandalone` in the same commit to refresh the standalone edition
- Any new `docs/*.html` documentation file → add it to the index table above

If a matching document exists and the commit changes behavior it describes (new bindings, new command groups, changed struct tags, altered command tree layout), update the HTML to reflect the current state **in the same commit**.
