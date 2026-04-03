// Package logging implements the "logging" command group.
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const dirName = ".botctrl/logging"

// Cmd is the Kong command group for "botctrl logging".
type Cmd struct {
	Clean cleanCmd `cmd:"clean" help:"Remove old log files from ~/.botctrl/logging/."`
}

type cleanCmd struct {
	All  bool `help:"Remove all log files regardless of age."`
	Days int  `help:"Remove log files older than N days." default:"7" placeholder:"N"`
}

func (c *cleanCmd) Run() error {
	dir, err := logDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading log directory: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -c.Days)
	var removed int

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !isLogFile(e.Name()) {
			continue
		}

		if !c.All {
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(cutoff) {
				continue
			}
		}

		path := filepath.Join(dir, e.Name())
		if err := os.Remove(path); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not remove %s: %v\n", e.Name(), err)
			continue
		}
		removed++
	}

	fmt.Fprintf(os.Stdout, "removed %d log file(s)\n", removed)
	return nil
}

// logDir returns the absolute path to ~/.botctrl/logging/.
func logDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, dirName), nil
}

// isLogFile returns true for files with a .jsonl extension (including rotated
// files like 20260403_post-tool-use.jsonl.1712345678).
func isLogFile(name string) bool {
	for {
		ext := filepath.Ext(name)
		if ext == ".jsonl" {
			return true
		}
		if ext == "" {
			return false
		}
		name = name[:len(name)-len(ext)]
	}
}
