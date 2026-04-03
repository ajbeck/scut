// Package claude implements the "claude" agent command group.
package claude

import (
	"io"
	"log/slog"
	"strings"

	"github.com/ajbeck/botctrl/internal/cmd/claude/hook"
	"github.com/ajbeck/botctrl/internal/logging"
)

// Cmd is the Kong command group for "botctrl claude".
type Cmd struct {
	Log      bool   `help:"Enable logging to ~/.botctrl/logging/ at warn level."`
	LogLevel string `help:"Set log level: debug, info, warn, error (implies --log)." placeholder:"LEVEL"`

	Hook       hook.Cmd      `cmd:"hook" help:"Hook event handlers. Called by Claude Code as subprocesses during lifecycle events."`
	StatusLine statusLineCmd `cmd:"status-line" help:"Render the Claude Code status bar. Reads session JSON from stdin, prints styled output to stdout."`
}

// OpenLogger returns a logger configured from the --log and --log-level flags.
// When logging is disabled, returns [logging.Discard] and a nil closer.
// The caller must close the returned [io.Closer] when done.
func (c *Cmd) OpenLogger(command string) (*slog.Logger, io.Closer) {
	if !c.Log && c.LogLevel == "" {
		return logging.Discard, nil
	}
	name := logName(command)
	logger, closer, err := logging.Open(name, c.resolveLevel())
	if err != nil {
		return logging.Discard, nil
	}
	return logger, closer
}

// resolveLevel converts the --log-level string to a slog.Level.
// Defaults to warn when --log is set without --log-level.
func (c *Cmd) resolveLevel() slog.Level {
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "error":
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}

// logName extracts the leaf command name from a kong command path.
// "claude hook post-tool-use" → "post-tool-use", "claude status-line" → "status-line".
func logName(command string) string {
	if i := strings.LastIndex(command, " "); i >= 0 {
		return command[i+1:]
	}
	return command
}
