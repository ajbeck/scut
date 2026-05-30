# scut

[![Release](https://img.shields.io/github/v/release/ajbeck/scut?sort=semver)](https://github.com/ajbeck/scut/releases)
[![Pull Request](https://github.com/ajbeck/scut/actions/workflows/pull-request.yaml/badge.svg)](https://github.com/ajbeck/scut/actions/workflows/pull-request.yaml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ajbeck/scut)](go.mod)

CLI tool for LLM agents. Provides a consistent interface for agent authors to interact with tools, the environment, and each other via hooks, rules, and instructions.

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/ajbeck/scut/main/install.sh | sh
```

By default, the installer downloads the latest GitHub Release for your platform,
verifies `checksums.txt`, and installs `scut` to `~/.local/bin`.

Install a specific version or destination:

```bash
curl -fsSL https://raw.githubusercontent.com/ajbeck/scut/main/install.sh | sh -s -- --version v0.1.0 --bin-dir /usr/local/bin
```

Go users can also install from source:

```bash
go install github.com/ajbeck/scut@latest
```

Or build locally:

```bash
mage build
```

The binary is written to `bin/scut`.

See [docs/installation.html](docs/installation.html) for release assets,
supported platforms, and install-script behavior.

## Hooks

scut implements [Claude Code hooks](https://docs.anthropic.com/en/docs/claude-code/hooks) as subcommands under `scut claude hook <event>`. Claude Code invokes these as subprocesses, piping JSON to stdin and reading JSON from stdout.

### PostToolUse — Auto-formatting

The `post-tool-use` hook automatically formats files after Claude's **Write** or **Edit** tool calls. It dispatches by file extension:

| Extension | Formatter | Notes |
|-----------|-----------|-------|
| `.go` | `gofmt` (via `go/format`) | Files with syntax errors are left unchanged |
| `.md`, `.mdx` | goldmark-prettier-markdown | Preserves prose wrapping style |

Files with other extensions are passed through unchanged.

### Configuration

Wire supported agents with the unified setup command:

```bash
scut init                         # detected agents, project scope
scut init --all --dry-run          # preview Claude Code + Codex setup
scut init --codex --scope=user     # explicitly set up Codex user hooks
scut init --all --bake-log-level=debug
```

Agent-specific commands remain available when you need lower-level control:

```bash
scut claude config install              # project scope: .claude/settings.json
scut claude config install --scope=user # user scope: ~/.claude/settings.json
scut claude config install --dry-run    # preview without writing
scut codex config install               # project scope: .codex/hooks.json
```

Claude setup installs entries for all 29 hook events plus the status line, merging non-destructively with any existing `settings.json`. Codex setup defaults to the `post-tool-use` formatter hook in `hooks.json`; use `--only` on `scut codex config install` to opt into additional Codex hook events. In project scope, `scut init` auto-detects agents only when `.claude/` or `.codex/` exists; pass `--all`, `--claude`, or `--codex` to force setup. See [docs/init-command.html](docs/init-command.html), [docs/config-command.html](docs/config-command.html), and [docs/codex-config-command.html](docs/codex-config-command.html) for flags, merge semantics, and the `uninstall` / `status` subcommands.

For reference, this is the PostToolUse entry the command writes:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write|Edit",
        "hooks": [
          {
            "type": "command",
            "command": "scut claude hook post-tool-use",
            "statusMessage": "Formatting..."
          }
        ]
      }
    ]
  }
}
```

**Fields:**

- **`matcher`** — filters which tool calls trigger the hook. `"Write|Edit"` matches either tool.
- **`type`** — handler type. Currently only `"command"` is supported.
- **`command`** — the CLI command Claude Code executes as a subprocess. Claude Code pipes the tool use event as JSON to stdin and reads the hook's JSON response from stdout.
- **`statusMessage`** — optional label shown in Claude Code's status line while the hook runs.

### How it works

1. Claude writes or edits a file using the Write or Edit tool.
2. Claude Code fires a `PostToolUse` event and pipes the event payload (including `tool_name`, `tool_input` with `file_path`, and `tool_response`) to the hook's stdin.
3. `scut claude hook post-tool-use` extracts the file path, checks the extension, and runs the appropriate formatter.
4. The formatted content is written back to the file in place. The hook returns an empty JSON response — formatting is silent and transparent to the agent.

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success. stdout is parsed as the hook's JSON response. |
| `2` | Blocking error. stderr is surfaced as an error message to the agent. |
| Other | Non-blocking error. Logged but execution continues. |

### Other hook events

scut has subcommands wired for all 29 Claude Code hook events and all 10 documented Codex command-hook events (e.g. `session-start`, `pre-tool-use`, `user-prompt-submit`, `stop`, etc.). Most non-formatting hooks currently deserialize input and return empty or placeholder responses. See the [Claude Code hooks documentation](https://docs.anthropic.com/en/docs/claude-code/hooks) and [Codex hooks documentation](https://developers.openai.com/codex/hooks) for the full event references.

```
scut claude hook --help
scut codex hook --help
```

## Status Line

`scut claude status-line` renders a persistent status bar at the bottom of the Claude Code terminal. It displays context window usage, model label, current path, git branch with dirty/ahead-behind indicators — all computed in-process with zero subprocess overhead.

```
████████████████│███ 50% | O4.6 | scut/internal/cmd | getting-started ✓ ↑1
```

| Segment | Description |
|---------|-------------|
| Context bar | 20-character progress bar with half-block resolution (38 levels). A red `│` marker sits at 83% — the auto-compaction threshold. Mint <70%, amber 70–82%, red 83%+. |
| Model | Abbreviated model label (e.g., `S4.5`, `O4.6-1M`) |
| Path | Current directory relative to the git repo root (or `~/relative` outside a repo). Long paths are compacted by collapsing intermediate segments. |
| Branch | Current git branch from HEAD, truncated to 20 characters |
| Git indicators | `✓` when clean, `+N` staged (mint), `~N` unstaged/untracked (amber), `↑N` ahead (mint), `↓N` behind (amber) |

### Configuration

The status line is wired by `scut claude config install` alongside the hooks. The entry it writes:

```json
{
  "statusLine": {
    "type": "command",
    "command": "scut claude status-line"
  }
}
```

To install only the status line without the hooks, pass `--only=status-line`:

```bash
scut claude config install --only=status-line
```

See [docs/config-command.html](docs/config-command.html) for the full surface.

### Performance

The status line fires after each assistant message (debounced at 300ms), when permission mode changes, or when vim mode toggles. scut is designed for this frequency:

- **No subprocesses**: git branch and worktree status are computed via [go-git](https://github.com/go-git/go-git) (pure Go), not by forking `git`.
- **Single repo open**: the `.git` directory is opened once per invocation and shared across all queries.
- **Concurrent collection**: path resolution, git status, ahead/behind counts, and context bar rendering run in parallel goroutines.
- **Styled via lipgloss**: ANSI escape codes are generated in-process using [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss).

## Logging

Enable structured JSONL logging for hook commands and the status line with the `--log` flag:

```bash
scut claude --log hook post-tool-use        # warn level (default)
scut claude --log-level=debug status-line   # debug level (implies --log)
```

Log files are written to `~/.scut/logging/` with date-partitioned filenames:

```
~/.scut/logging/
  20260403_post-tool-use.jsonl
  20260403_status-line.jsonl
```

Each line is a JSON object with standardized fields: `time`, `level`, `msg`, `hook`, `session_id`, `duration_ms`, plus hook-specific attributes.

To bake logging into the installed hook commands, pass `--log` (or `--log-level=LEVEL`) when installing:

```bash
scut claude config install --log
scut claude config install --log-level=debug
```

The generated command in `settings.json` becomes `scut claude --log hook post-tool-use` (or `--log-level=debug`).

### Cleanup

Remove old log files with `scut logging clean`:

```bash
scut logging clean              # remove files older than 7 days (default)
scut logging clean --days 30    # remove files older than 30 days
scut logging clean --all        # remove all log files
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development commands and pull request expectations.

## Security

Report security issues privately. See [SECURITY.md](SECURITY.md).

## License

scut is released under the [MIT License](LICENSE).
