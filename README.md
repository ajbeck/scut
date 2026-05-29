# scut

CLI tool for LLM agents. Provides a consistent interface for agent authors to interact with tools, the environment, and each other via hooks, rules, and instructions.

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/ajbeck/scut/main/install.sh | sh
```

By default, the installer downloads the latest GitHub Release for your platform,
verifies `checksums.txt`, and installs `scut` to `~/.local/bin`.

Install a specific version or destination:

```bash
curl -fsSL https://raw.githubusercontent.com/ajbeck/scut/main/install.sh | sh -s -- --version v0.2.0 --bin-dir /usr/local/bin
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

Wire everything up with a single command:

```bash
scut claude config install              # project scope: .claude/settings.json
scut claude config install --scope=user # user scope: ~/.claude/settings.json
scut claude config install --dry-run    # preview without writing
```

This installs entries for all 25 hook events plus the status line, merging non-destructively with any existing `settings.json`. See [docs/config-command.html](docs/config-command.html) for flags, merge semantics, and the `uninstall` / `status` subcommands.

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

scut has subcommands wired for all 25 Claude Code hook events (e.g. `session-start`, `pre-tool-use`, `user-prompt-submit`, `stop`, etc.). These are currently stub implementations that deserialize input and return empty responses. See the [Claude Code hooks documentation](https://docs.anthropic.com/en/docs/claude-code/hooks) for the full event reference.

```
scut claude hook --help
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
