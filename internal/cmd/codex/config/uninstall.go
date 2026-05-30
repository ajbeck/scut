//go:build goexperiment.jsonv2

package config

import (
	"fmt"
	"io"
	"log/slog"
)

import "github.com/spf13/afero"

// uninstallCmd is the kong leaf for "scut codex config uninstall".
type uninstallCmd struct {
	Scope  string   `help:"Hooks scope: project or user." default:"project" enum:"project,user"`
	Only   []string `help:"Comma-separated list of hook event slugs to remove. Defaults to all scut-owned hook events." sep:","`
	DryRun bool     `help:"Print resulting hooks.json to stdout instead of writing." name:"dry-run"`
}

func (c *uninstallCmd) Run(stdout io.Writer, stderr io.Writer, fs afero.Fs, logger *slog.Logger) error {
	path, err := resolveScope(c.Scope)
	if err != nil {
		return err
	}

	if isNotExist(fs, path) {
		fmt.Fprintf(stderr, "no hooks file at %q; nothing to remove\n", path)
		return nil
	}

	h, err := readHooksFile(fs, path)
	if err != nil {
		return err
	}

	removeSet, err := resolveRemoveSet(c.Only)
	if err != nil {
		return err
	}

	eventsToRemove := make(map[string]bool)
	for slug := range removeSet {
		spec, ok := hookSpecBySlug(slug)
		if ok {
			eventsToRemove[spec.Event] = true
		}
	}

	for event, groups := range h.Hooks {
		if !eventsToRemove[event] {
			continue
		}
		var kept []HookGroup
		for _, g := range groups {
			if !isScutGroup(g) {
				kept = append(kept, g)
			}
		}
		if len(kept) == 0 {
			delete(h.Hooks, event)
		} else {
			h.Hooks[event] = kept
		}
	}

	if len(h.Hooks) == 0 {
		h.Hooks = nil
	}

	if c.DryRun {
		data, err := marshalHooksFile(h)
		if err != nil {
			return err
		}
		_, err = stdout.Write(data)
		return err
	}

	if err := writeHooksFile(fs, path, h); err != nil {
		return err
	}

	logger.Info("uninstalled", "scope", c.Scope, "path", path)
	return nil
}
