//go:build goexperiment.jsonv2

package config

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/spf13/afero"
)

// uninstallCmd is the kong leaf for "botctrl claude config uninstall".
type uninstallCmd struct {
	Scope  string   `help:"Settings scope: project or user." default:"project" enum:"project,user"`
	Only   []string `help:"Comma-separated list of items to remove (hook event slugs and/or 'status-line')." sep:","`
	DryRun bool     `help:"Print resulting JSON to stdout instead of writing." name:"dry-run"`
}

// Run executes the uninstall command: removes botctrl entries from settings.json.
func (c *uninstallCmd) Run(stdout io.Writer, stderr io.Writer, fs afero.Fs, logger *slog.Logger) error {
	// Resolve the settings file path.
	path, err := resolveScope(c.Scope)
	if err != nil {
		return err
	}

	// If the file doesn't exist, report and exit 0.
	if isNotExist(fs, path) {
		fmt.Fprintf(stderr, "no settings file at %q; nothing to remove\n", path)
		return nil
	}

	// Read and unmarshal.
	s, err := readSettings(fs, path)
	if err != nil {
		return err
	}

	// Compute the remove set.
	removeSet, err := resolveInstallSet(c.Only)
	if err != nil {
		return err
	}

	// Remove statusLine if owned and status-line is in the remove set.
	if removeSet["status-line"] && s.StatusLine != nil && owns(s.StatusLine.Command) {
		s.StatusLine = nil
	}

	// Build the set of event names to process based on the remove set slugs.
	eventsToRemove := make(map[string]bool)
	for slug := range removeSet {
		if slug == "status-line" {
			continue
		}
		spec, ok := hookSpecBySlug(slug)
		if ok {
			eventsToRemove[spec.Event] = true
		}
	}

	// For each event in the hooks map, remove botctrl-owned groups when the event is targeted.
	for event, groups := range s.Hooks {
		if !eventsToRemove[event] {
			continue
		}
		var kept []HookGroup
		for _, g := range groups {
			if !isBotctrlGroup(g) {
				kept = append(kept, g)
			}
		}
		if len(kept) == 0 {
			delete(s.Hooks, event)
		} else {
			s.Hooks[event] = kept
		}
	}

	// If the hooks map is now empty, delete it.
	if len(s.Hooks) == 0 {
		s.Hooks = nil
	}

	// Dry-run: write JSON to stdout and return.
	if c.DryRun {
		data, err := marshalSettings(s)
		if err != nil {
			return err
		}
		_, err = stdout.Write(data)
		return err
	}

	// Write to file.
	if err := writeSettings(fs, path, s); err != nil {
		return err
	}

	logger.Info("uninstalled",
		"scope", c.Scope,
		"path", path,
	)
	return nil
}

// resolveScopePaths returns one or more settings file paths for the given scope string.
// Used by the status command which supports "both".
func resolveScopePaths(scope string) ([]string, error) {
	switch scope {
	case "project":
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolving cwd: %w", err)
		}
		return []string{projectSettingsPath(cwd)}, nil
	case "user":
		p, err := userSettingsPath()
		if err != nil {
			return nil, err
		}
		return []string{p}, nil
	case "both":
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolving cwd: %w", err)
		}
		proj := projectSettingsPath(cwd)
		user, err := userSettingsPath()
		if err != nil {
			return nil, err
		}
		return []string{proj, user}, nil
	default:
		return nil, fmt.Errorf("unknown scope %q", scope)
	}
}
