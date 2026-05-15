//go:build goexperiment.jsonv2

package config

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/spf13/afero"
)

// installCmd is the kong leaf for "scut claude config install".
type installCmd struct {
	Scope        string   `help:"Settings scope: project or user." default:"project" enum:"project,user"`
	Only         []string `help:"Comma-separated list of items to install (hook event slugs and/or 'status-line')." sep:","`
	BakeLog      bool     `help:"Bake --log into generated command strings." name:"bake-log"`
	BakeLogLevel string   `help:"Bake --log-level=LEVEL into generated command strings (implies --bake-log). One of: debug, info, warn, error." placeholder:"LEVEL" name:"bake-log-level" enum:",debug,info,warn,error" default:""`
	DryRun       bool     `help:"Print resulting JSON to stdout instead of writing." name:"dry-run"`
}

// Run executes the install command: reads settings.json, merges scut entries, and writes back.
func (c *installCmd) Run(stdout io.Writer, fs afero.Fs, logger *slog.Logger) error {
	// Resolve the settings file path.
	path, err := resolveScope(c.Scope)
	if err != nil {
		return err
	}

	// Read existing settings (ENOENT → empty Settings{}).
	s, err := readSettings(fs, path)
	if err != nil {
		return err
	}

	// Compute the install set.
	installSet, err := resolveInstallSet(c.Only)
	if err != nil {
		return err
	}

	// Check for foreign statusLine conflict before writing anything.
	if installSet["status-line"] && s.StatusLine != nil && !owns(s.StatusLine.Command) {
		return fmt.Errorf("%w at %q: pass --only without status-line to skip, or remove it manually first",
			ErrForeignStatusLine, path)
	}

	// Build the command string prefix based on bake-log flags.
	logPrefix := buildLogPrefix(c.BakeLog, c.BakeLogLevel)

	// Apply each item in the install set.
	if installSet["status-line"] {
		s.StatusLine = &StatusLine{
			Type:    "command",
			Command: "scut claude " + logPrefix + "status-line",
		}
	}

	for _, spec := range hookSpecs {
		if !installSet[spec.Slug] {
			continue
		}
		cmd := "scut claude " + logPrefix + "hook " + spec.Slug
		entry := HookEntry{
			Type:          "command",
			Command:       cmd,
			StatusMessage: spec.StatusMessage,
		}
		group := HookGroup{
			Matcher: spec.Matcher,
			Hooks:   []HookEntry{entry},
		}
		if s.Hooks == nil {
			s.Hooks = make(map[string][]HookGroup)
		}
		s.Hooks[spec.Event] = mergeHookGroup(s.Hooks[spec.Event], group)
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

	logger.Info("installed",
		"scope", c.Scope,
		"path", path,
		"entries", len(installSet),
		"bake_log", c.BakeLog || c.BakeLogLevel != "",
	)
	return nil
}

// resolveScope returns the settings file path for the given scope string.
func resolveScope(scope string) (string, error) {
	switch scope {
	case "project":
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolving cwd: %w", err)
		}
		return projectSettingsPath(cwd), nil
	case "user":
		return userSettingsPath()
	default:
		return "", fmt.Errorf("unknown scope %q", scope)
	}
}

// resolveInstallSet converts a list of --only tokens to a set of slugs to install.
// An empty list installs all 25 hook slugs plus "status-line".
func resolveInstallSet(only []string) (map[string]bool, error) {
	if len(only) == 0 {
		// Install everything.
		set := make(map[string]bool, len(hookSpecs)+1)
		for _, s := range hookSpecs {
			set[s.Slug] = true
		}
		set["status-line"] = true
		return set, nil
	}

	// Validate each token.
	valid := validTokenSet()
	set := make(map[string]bool, len(only))
	for _, tok := range only {
		if !valid[tok] {
			sorted := sortedValidTokens()
			return nil, fmt.Errorf("%w %q; valid tokens: %s",
				ErrUnknownOnlyToken, tok, strings.Join(sorted, ", "))
		}
		set[tok] = true
	}
	return set, nil
}

// validTokenSet returns a map of all valid --only tokens.
func validTokenSet() map[string]bool {
	m := make(map[string]bool, len(hookSpecs)+1)
	for _, s := range hookSpecs {
		m[s.Slug] = true
	}
	m["status-line"] = true
	return m
}

// sortedValidTokens returns all valid --only tokens in alphabetical order.
func sortedValidTokens() []string {
	m := validTokenSet()
	tokens := make([]string, 0, len(m))
	for t := range m {
		tokens = append(tokens, t)
	}
	sort.Strings(tokens)
	return tokens
}

// buildLogPrefix returns the command-string fragment to insert between "scut claude "
// and the next token, based on the bake-log flags.
//
// Neither flag:     "" (empty — bare "scut claude hook <slug>")
// --bake-log only:  "--log "
// --bake-log-level: "--log-level=LEVEL " (bare --log is redundant per OpenLogger semantics)
func buildLogPrefix(bakeLog bool, bakeLogLevel string) string {
	if bakeLogLevel != "" {
		return "--log-level=" + bakeLogLevel + " "
	}
	if bakeLog {
		return "--log "
	}
	return ""
}

// mergeHookGroup inserts scut's group at index 0 of groups, replacing any
// existing scut group and preserving foreign groups in their original order.
func mergeHookGroup(groups []HookGroup, scutGroup HookGroup) []HookGroup {
	// Remove any existing scut-owned group.
	foreign := make([]HookGroup, 0, len(groups))
	for _, g := range groups {
		if !isScutGroup(g) {
			foreign = append(foreign, g)
		}
	}
	// Insert scut's group at index 0.
	return append([]HookGroup{scutGroup}, foreign...)
}

// isScutGroup reports whether every inner hooks[].command in g is scut-owned.
// A mixed group (any foreign command) is not scut-owned.
func isScutGroup(g HookGroup) bool {
	if len(g.Hooks) == 0 {
		return false
	}
	for _, h := range g.Hooks {
		if !owns(h.Command) {
			return false
		}
	}
	return true
}
