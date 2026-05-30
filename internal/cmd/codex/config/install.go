//go:build goexperiment.jsonv2

package config

import (
	"fmt"
	"io"
	"log/slog"
)

import "github.com/spf13/afero"

// InstallOptions configures Codex hook installation.
type InstallOptions struct {
	Scope        string
	Only         []string
	BakeLog      bool
	BakeLogLevel string
	DryRun       bool
}

// installCmd is the kong leaf for "scut codex config install".
type installCmd struct {
	Scope        string   `help:"Hooks scope: project or user." default:"project" enum:"project,user"`
	Only         []string `help:"Comma-separated list of hook event slugs to install. Defaults to post-tool-use." sep:","`
	BakeLog      bool     `help:"Bake --log into generated command strings." name:"bake-log"`
	BakeLogLevel string   `help:"Bake --log-level=LEVEL into generated command strings (implies --bake-log). One of: debug, info, warn, error." placeholder:"LEVEL" name:"bake-log-level" enum:",debug,info,warn,error" default:""`
	DryRun       bool     `help:"Print resulting hooks.json to stdout instead of writing." name:"dry-run"`
}

func (c *installCmd) Run(stdout io.Writer, fs afero.Fs, logger *slog.Logger) error {
	return Install(stdout, fs, logger, InstallOptions{
		Scope:        c.Scope,
		Only:         c.Only,
		BakeLog:      c.BakeLog,
		BakeLogLevel: c.BakeLogLevel,
		DryRun:       c.DryRun,
	})
}

// Install reads hooks.json, merges scut entries, and writes the result.
func Install(stdout io.Writer, fs afero.Fs, logger *slog.Logger, opts InstallOptions) error {
	if opts.Scope == "" {
		opts.Scope = "project"
	}
	path, err := resolveScope(opts.Scope)
	if err != nil {
		return err
	}

	h, err := readHooksFile(fs, path)
	if err != nil {
		return err
	}

	installSet, err := resolveInstallSet(opts.Only)
	if err != nil {
		return err
	}

	logPrefix := buildLogPrefix(opts.BakeLog, opts.BakeLogLevel)
	for _, spec := range hookSpecs {
		if !installSet[spec.Slug] {
			continue
		}
		entry := HookEntry{
			Type:          "command",
			Command:       "scut codex " + logPrefix + "hook " + spec.Slug,
			StatusMessage: spec.StatusMessage,
		}
		group := HookGroup{
			Matcher: spec.Matcher,
			Hooks:   []HookEntry{entry},
		}
		if h.Hooks == nil {
			h.Hooks = make(map[string][]HookGroup)
		}
		h.Hooks[spec.Event] = mergeHookGroup(h.Hooks[spec.Event], group)
	}

	if opts.DryRun {
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

	logger.Info("installed",
		"scope", opts.Scope,
		"path", path,
		"entries", len(installSet),
		"bake_log", opts.BakeLog || opts.BakeLogLevel != "",
	)
	return nil
}

func buildLogPrefix(bakeLog bool, bakeLogLevel string) string {
	if bakeLogLevel != "" {
		return "--log-level=" + bakeLogLevel + " "
	}
	if bakeLog {
		return "--log "
	}
	return ""
}

// PathForScope returns the hooks.json path for a project or user scope.
func PathForScope(scope string) (string, error) {
	path, err := resolveScope(scope)
	if err != nil {
		return "", fmt.Errorf("resolving Codex hooks path: %w", err)
	}
	return path, nil
}
