// Package logging provides structured JSONL logging for scut commands.
// Log files are written to ~/.scut/logging/ with date and component
// name in the filename. Files are rotated on open when they exceed 10 MB.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const (
	maxFileSize = 10 * 1024 * 1024 // 10 MB — rotate when exceeded
	dirName     = ".scut/logging"
)

// Discard is a no-op logger that discards all output. Use it when logging
// is disabled so callers don't need nil checks.
var Discard = slog.New(slog.NewTextHandler(io.Discard, nil))

// Open creates or opens a JSONL log file for the named component (e.g.
// "post-tool-use", "status-line"). The file is placed at
// ~/.scut/logging/YYYYMMDD_<name>.jsonl.
//
// If the file already exists and exceeds maxFileSize, it is rotated
// (renamed with a unix-second suffix) before opening a fresh file.
//
// The returned [io.Closer] must be called when logging is complete.
func Open(name string, level slog.Level) (*slog.Logger, io.Closer, error) {
	dir, err := logDir()
	if err != nil {
		return nil, nil, err
	}

	path := filepath.Join(dir, fileName(name))

	if err := rotateIfNeeded(path); err != nil {
		return nil, nil, err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: level,
	}))

	return logger, f, nil
}

// logDir returns the absolute path to ~/.scut/logging/, creating it
// if necessary.
func logDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	dir := filepath.Join(home, dirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create log directory: %w", err)
	}
	return dir, nil
}

// fileName returns the log file name for today and the given component.
func fileName(name string) string {
	return time.Now().Format("20060102") + "_" + name + ".jsonl"
}

// LogParseError appends a JSONL record describing a kong parse failure
// to ~/.scut/logging/YYYYMMDD_parse-errors.jsonl. It is unconditional —
// parse errors are always bugs worth capturing, so no flag gates them.
// Failure to write is swallowed: losing the record must not prevent the
// parent process from seeing the original kong error on stderr.
func LogParseError(args []string, parseErr error) {
	logger, closer, err := Open("parse-errors", slog.LevelError)
	if err != nil {
		return
	}
	defer closer.Close()
	logger.Error("parse failed",
		"args", args,
		"argc", len(args),
		"error", parseErr.Error(),
	)
}

// rotateIfNeeded renames path to path.<unix-seconds> if it exists and
// exceeds maxFileSize. Does nothing if the file is small or missing.
func rotateIfNeeded(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return nil // file doesn't exist yet — nothing to rotate
	}
	if info.Size() <= maxFileSize {
		return nil
	}
	rotated := path + "." + fmt.Sprintf("%d", time.Now().Unix())
	if err := os.Rename(path, rotated); err != nil {
		return fmt.Errorf("rotate log file: %w", err)
	}
	return nil
}
