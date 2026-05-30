//go:build goexperiment.jsonv2

// Package codex implements the "codex" agent command group.
package codex

import (
	"io"
	"log/slog"
	"strings"

	"github.com/ajbeck/scut/internal/cmd/codex/config"
	"github.com/ajbeck/scut/internal/cmd/codex/hook"
	"github.com/ajbeck/scut/internal/logging"
)

// Cmd is the Kong command group for "scut codex".
type Cmd struct {
	Log      bool   `help:"Enable logging to ~/.scut/logging/ at info level."`
	LogLevel string `help:"Set log level: debug, info, warn, error (implies --log)." placeholder:"LEVEL"`

	Hook   hook.Cmd   `cmd:"hook" help:"Hook event handlers. Called by Codex as subprocesses during lifecycle events."`
	Config config.Cmd `cmd:"config" help:"Configure Codex hooks.json — install or remove scut hooks."`
}

// OpenLogger returns a logger configured from the --log and --log-level flags.
func (c *Cmd) OpenLogger(command string) (*slog.Logger, io.Closer) {
	if !c.Log && c.LogLevel == "" {
		return logging.Discard, nil
	}
	logger, closer, err := logging.Open(logName(command), c.resolveLevel())
	if err != nil {
		return logging.Discard, nil
	}
	return logger, closer
}

func (c *Cmd) resolveLevel() slog.Level {
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func logName(command string) string {
	if i := strings.LastIndex(command, " "); i >= 0 {
		return command[i+1:]
	}
	return command
}
