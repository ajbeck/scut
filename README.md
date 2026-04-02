# botctrl

CLI tool for LLM agents. Provides a consistent interface for agent authors to interact with tools, the environment, and each other via hooks, rules, and instructions.

## Installation

```bash
go install github.com/ajbeck/botctrl@latest
```

Or build from source:

```bash
mage build
```

The binary is written to `bin/botctrl`.

## Hooks

botctrl implements [Claude Code hooks](https://docs.anthropic.com/en/docs/claude-code/hooks) as subcommands under `botctrl claude hook <event>`. Claude Code invokes these as subprocesses, piping JSON to stdin and reading JSON from stdout.

### PostToolUse — Auto-formatting

The `post-tool-use` hook automatically formats files after Claude's **Write** or **Edit** tool calls. It dispatches by file extension:

| Extension | Formatter | Notes |
|-----------|-----------|-------|
| `.go` | `gofmt` (via `go/format`) | Files with syntax errors are left unchanged |
| `.md`, `.mdx` | goldmark-prettier-markdown | Preserves prose wrapping style |

Files with other extensions are passed through unchanged.

### Configuration

Add the following to your Claude Code `settings.json` (project-level at `.claude/settings.json` or user-level at `~/.claude/settings.json`):

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write",
        "hooks": [
          {
            "type": "command",
            "command": "botctrl claude hook post-tool-use",
            "statusMessage": "Formatting..."
          }
        ]
      },
      {
        "matcher": "Edit",
        "hooks": [
          {
            "type": "command",
            "command": "botctrl claude hook post-tool-use",
            "statusMessage": "Formatting..."
          }
        ]
      }
    ]
  }
}
```

**Fields:**

- **`matcher`** — filters which tool calls trigger the hook. Set to `"Write"` or `"Edit"` to match the corresponding Claude Code tool.
- **`type`** — handler type. Currently only `"command"` is supported.
- **`command`** — the CLI command Claude Code executes as a subprocess. Claude Code pipes the tool use event as JSON to stdin and reads the hook's JSON response from stdout.
- **`statusMessage`** — optional label shown in Claude Code's status line while the hook runs.

### How it works

1. Claude writes or edits a file using the Write or Edit tool.
2. Claude Code fires a `PostToolUse` event and pipes the event payload (including `tool_name`, `tool_input` with `file_path`, and `tool_response`) to the hook's stdin.
3. `botctrl claude hook post-tool-use` extracts the file path, checks the extension, and runs the appropriate formatter.
4. The formatted content is written back to the file in place. The hook returns an empty JSON response — formatting is silent and transparent to the agent.

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success. stdout is parsed as the hook's JSON response. |
| `2` | Blocking error. stderr is surfaced as an error message to the agent. |
| Other | Non-blocking error. Logged but execution continues. |

### Other hook events

botctrl has subcommands wired for all 25 Claude Code hook events (e.g. `session-start`, `pre-tool-use`, `user-prompt-submit`, `stop`, etc.). These are currently stub implementations that deserialize input and return empty responses. See the [Claude Code hooks documentation](https://docs.anthropic.com/en/docs/claude-code/hooks) for the full event reference.

```
botctrl claude hook --help
```

## Status Line

`botctrl claude status-line` renders a persistent status bar at the bottom of the Claude Code terminal. It displays the current path, git branch with dirty indicators, and a context window usage bar — all computed in-process with zero subprocess overhead.

```
botctrl/internal/cmd | getting-started +2 ~5 | ██░░░░░░░░ 25%
```

| Segment | Description |
|---------|-------------|
| Path | Current directory relative to the git repo root (or `~/relative` outside a repo) |
| Branch | Current git branch from HEAD |
| Dirty indicators | `+N` staged (green), `~N` unstaged/untracked (amber) |
| Context bar | 10-character progress bar with percentage. Green <70%, amber 70–89%, red 90%+ |

### Configuration

Add a `statusLine` entry to your Claude Code `settings.json`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "botctrl claude status-line"
  }
}
```

This can live alongside hooks in the same settings file. A complete configuration with both the status line and the post-tool-use formatting hook:

```json
{
  "statusLine": {
    "type": "command",
    "command": "botctrl claude status-line"
  },
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write",
        "hooks": [
          {
            "type": "command",
            "command": "botctrl claude hook post-tool-use",
            "statusMessage": "Formatting..."
          }
        ]
      },
      {
        "matcher": "Edit",
        "hooks": [
          {
            "type": "command",
            "command": "botctrl claude hook post-tool-use",
            "statusMessage": "Formatting..."
          }
        ]
      }
    ]
  }
}
```

### Performance

The status line runs after every assistant message (debounced at 300ms). botctrl is designed for this frequency:

- **No subprocesses**: git branch and worktree status are computed via [go-git](https://github.com/go-git/go-git) (pure Go), not by forking `git`.
- **Single repo open**: the `.git` directory is opened once per invocation and shared across all queries.
- **Concurrent collection**: path resolution, git status, and context bar rendering run in parallel goroutines.
- **Styled via lipgloss**: ANSI escape codes are generated in-process using [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) with automatic colour profile detection.
