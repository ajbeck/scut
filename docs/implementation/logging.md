# Logging

`botctrl claude --log` | `botctrl claude --log-level=LEVEL`

## Overview

Structured JSONL logging for all hook commands and the status line. When enabled, log lines are appended to date-partitioned files at `~/.botctrl/logging/`. Logging is off by default — hooks run silently unless a flag is set.

## Flags

Flags live on `claude.Cmd` so they propagate to both `hook` subcommands and `status-line`.

| Flag          | Type     | Default | Behavior                                                          |
| ------------- | -------- | ------- | ----------------------------------------------------------------- |
| `--log`       | `bool`   | `false` | Enable logging at info level                                      |
| `--log-level` | `string` | —     | Set log level (`debug`, `info`, `warn`, `error`); implies `--log` |

Info is the default because the hook/status-line "happy path" logs at info; a warn default would produce empty files in normal operation. Use `--log-level=debug` to capture full status-line input payloads (model ID, context window size, workspace dir).

When neither flag is set, all `Run()` methods receive `logging.Discard` — a no-op logger backed by `io.Discard`. No file is opened, no disk I/O occurs.

### Parse-error logging (unconditional)

`main()` wraps kong parsing in two steps — `kong.Must()` then `parser.Parse(os.Args[1:])` — so we can intercept parse failures before kong calls `os.Exit`. When parsing fails, `logging.LogParseError(os.Args, err)` appends a record to `~/.botctrl/logging/YYYYMMDD_parse-errors.jsonl` capturing the full `os.Args` (including argv[0]) and the kong error message. This happens regardless of flags — parse errors are always bugs worth recording, and they occur before the `--log` / `--log-level` flags have been parsed out of the args anyway.

When parsing succeeds, `main()` emits a `logger.Debug("invoked", "args", os.Args, "command", ctx.Command())` record so the success path can be compared against failures.

## File Layout

```
~/.botctrl/logging/
  20260403_post-tool-use.jsonl     ← today's post-tool-use log
  20260403_status-line.jsonl       ← today's status-line log
  20260403_parse-errors.jsonl      ← kong parse failures (written unconditionally)
  20260402_post-tool-use.jsonl     ← yesterday's log
  20260401_session-start.jsonl.1712000000  ← rotated file
```

- **Filename**: `YYYYMMDD_<command-name>.jsonl` where command-name is the leaf of the Kong command path (e.g., `post-tool-use`, `status-line`, `session-start`).
- **Format**: JSONL — one JSON object per line, produced by `slog.NewJSONHandler`.
- **Rotation**: On open, if the file exceeds 10 MB it is renamed with a unix-second suffix (e.g., `.jsonl.1712345678`) and a fresh file is created. Rotation happens at most once per process invocation.
- **Write mode**: `O_APPEND|O_CREATE|O_WRONLY` — the file is never read into memory. POSIX guarantees atomicity for small writes, so concurrent hook invocations writing to the same file don't corrupt each other.

## Architecture

### Wiring

The logger is created in `main()` after parsing but before `Run()`:

```go
ctx, err := parser.Parse(os.Args[1:])
if err != nil {
    logging.LogParseError(os.Args, err)
    parser.FatalIfErrorf(err)
}

logger, logCloser := c.Claude.OpenLogger(ctx.Command())
if logCloser != nil {
    defer logCloser.Close()
}

logger.Debug("invoked", "args", os.Args, "command", ctx.Command())

ctx.FatalIfErrorf(ctx.Run(logger))
```

`ctx.Run(logger)` passes the `*slog.Logger` as an extra binding. Kong injects it into any `Run()` method that declares a `*slog.Logger` parameter.

### OpenLogger

`claude.Cmd.OpenLogger(command string)` checks the `--log` and `--log-level` flags. If logging is disabled, it returns `logging.Discard` and a nil closer. If enabled, it calls `logging.Open(name, level)` which:

1. Resolves `~/.botctrl/logging/` (creating it if needed)
2. Computes the filename from today's date and the command name
3. Rotates the existing file if it exceeds 10 MB
4. Opens the file in append mode
5. Returns a `*slog.Logger` backed by `slog.NewJSONHandler`

The command name is extracted from the Kong command path: `"claude hook post-tool-use"` → `"post-tool-use"`, `"claude status-line"` → `"status-line"`.

### Run method signatures

All hook commands and the status line accept `*slog.Logger` as a `Run()` parameter:

```go
// Hook commands (most):
func (c *sessionStartCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error

// PostToolUse (also needs afero.Fs):
func (c *postToolUseCmd) Run(stdin io.Reader, stdout io.Writer, fs afero.Fs, logger *slog.Logger) error

// Status line:
func (c *statusLineCmd) Run(stdin io.Reader, stdout io.Writer, logger *slog.Logger) error
```

In tests, pass `logging.Discard` as the logger:

```go
cmd := &postToolUseCmd{}
err := cmd.Run(stdin, &stdout, fs, logging.Discard)
```

## Standardized Log Fields

Every log line includes these fields (via `slog.JSONHandler`):

| Field         | Source  | Description                                                         |
| ------------- | ------- | ------------------------------------------------------------------- |
| `time`        | slog    | ISO 8601 timestamp                                                  |
| `level`       | slog    | INFO, WARN, ERROR, DEBUG                                            |
| `msg`         | handler | Action taken: `"handled"`, `"formatted"`, `"rendered"`, `"skipped"` |
| `hook`        | handler | Hook/command name: `"post-tool-use"`, `"status-line"`, etc.         |
| `session_id`  | input   | Claude Code session identifier                                      |
| `duration_ms` | handler | Wall-clock milliseconds for the handler                             |

### Hook-specific fields

| Hook                          | Extra Fields                                                                                                                                                    |
| ----------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `session-start`               | `source` (startup, resume, clear, compact)                                                                                                                      |
| `session-end`                 | `reason` (clear, resume, logout, etc.)                                                                                                                          |
| `pre-tool-use`                | `tool_name`                                                                                                                                                     |
| `post-tool-use`               | `tool_name`, `file_path`, `formatter`, `reason` (when skipped)                                                                                                  |
| `stop-failure`                | `error` (rate_limit, server_error, etc.)                                                                                                                        |
| `pre-compact`, `post-compact` | `trigger` (manual, auto)                                                                                                                                        |
| `status-line`                 | `model` (raw ID), `path`, `branch`, `context_pct`; at debug: `model_display_name`, `context_window_size`, `exceeds_200k_tokens`, `cwd`, `workspace_current_dir` |
| `parse-errors`                | `args` (full `os.Args`), `argc`, `error` (kong error message); written unconditionally on parse failure regardless of `--log` flag                              |
| (every command, at debug)     | `"invoked"` record with `args` (full `os.Args`) and `command` (resolved kong command path); emitted from `main()` before dispatch                               |

## Cleanup

`botctrl logging clean` removes old log files.

| Flag       | Default | Behavior                                    |
| ---------- | ------- | ------------------------------------------- |
| `--all`    | `false` | Remove all `.jsonl` files regardless of age |
| `--days N` | `7`     | Remove files with mtime older than N days   |

The command scans `~/.botctrl/logging/` and removes files with a `.jsonl` extension (including rotated files like `.jsonl.1712345678`). Non-`.jsonl` files are left untouched.

## Code

- **Core package**: `internal/logging/logging.go` — `Open`, `Discard`, rotation logic
- **Flags + OpenLogger**: `internal/cmd/claude/claude.go`
- **Clean command**: `internal/cmd/logging/logging.go`
- **Main wiring**: `cmd/botctrl/main.go`
