//go:build goexperiment.jsonv2

package config

import (
	"fmt"
	"io"
	"log/slog"

	json "encoding/json/v2"

	"github.com/spf13/afero"
)

// statusCmd is the kong leaf for "scut codex config status".
type statusCmd struct {
	Scope string `help:"Which scope(s) to inspect." default:"both" enum:"project,user,both"`
	JSON  bool   `help:"Emit a structured JSON object instead of the human-readable table." name:"json"`
}

type statusEntry struct {
	Kind    string `json:"kind"`
	Event   string `json:"event"`
	Matcher string `json:"matcher,omitzero"`
	Command string `json:"command"`
}

type scopeResult struct {
	Scope   string        `json:"scope"`
	Path    string        `json:"path"`
	Exists  bool          `json:"exists"`
	Entries []statusEntry `json:"entries"`
}

type statusOutput struct {
	Scopes []scopeResult `json:"scopes"`
}

func (c *statusCmd) Run(stdout io.Writer, fs afero.Fs, logger *slog.Logger) error {
	paths, err := resolveScopePaths(c.Scope)
	if err != nil {
		return err
	}
	scopeNames := scopeNamesForPaths(c.Scope, len(paths))

	results := make([]scopeResult, 0, len(paths))
	for i, path := range paths {
		sr, err := inspectScope(fs, scopeNames[i], path)
		if err != nil {
			return err
		}
		results = append(results, sr)
	}

	if c.JSON {
		return writeStatusJSON(stdout, statusOutput{Scopes: results})
	}
	return writeStatusHuman(stdout, results)
}

func scopeNamesForPaths(scope string, count int) []string {
	switch scope {
	case "both":
		return []string{"project", "user"}
	default:
		names := make([]string, count)
		for i := range names {
			names[i] = scope
		}
		return names
	}
}

func inspectScope(fs afero.Fs, scope, path string) (scopeResult, error) {
	exists := !isNotExist(fs, path)
	sr := scopeResult{
		Scope:   scope,
		Path:    path,
		Exists:  exists,
		Entries: []statusEntry{},
	}
	if !exists {
		return sr, nil
	}

	h, err := readHooksFile(fs, path)
	if err != nil {
		return sr, err
	}

	for _, spec := range hookSpecs {
		groups, ok := h.Hooks[spec.Event]
		if !ok {
			continue
		}
		for _, g := range groups {
			if !isScutGroup(g) {
				continue
			}
			for _, hook := range g.Hooks {
				sr.Entries = append(sr.Entries, statusEntry{
					Kind:    "hook",
					Event:   spec.Event,
					Matcher: g.Matcher,
					Command: hook.Command,
				})
			}
		}
	}

	return sr, nil
}

func writeStatusJSON(w io.Writer, out statusOutput) error {
	data, err := json.Marshal(out, json.Deterministic(true))
	if err != nil {
		return fmt.Errorf("marshalling status JSON: %w", err)
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

func writeStatusHuman(w io.Writer, results []scopeResult) error {
	for _, r := range results {
		fmt.Fprintf(w, "%-8s %s\n", scopeLabel(r.Scope), r.Path)
		if !r.Exists {
			fmt.Fprintf(w, "  (file does not exist)\n")
			continue
		}
		if len(r.Entries) == 0 {
			fmt.Fprintf(w, "  (no scut entries)\n")
			continue
		}
		for _, e := range r.Entries {
			if e.Matcher != "" && e.Matcher != "*" {
				fmt.Fprintf(w, "  %-20s %s   (matcher: %s)\n", e.Event, e.Command, e.Matcher)
			} else {
				fmt.Fprintf(w, "  %-20s %s\n", e.Event, e.Command)
			}
		}
	}
	return nil
}

func scopeLabel(scope string) string {
	switch scope {
	case "project":
		return "PROJECT"
	case "user":
		return "USER"
	default:
		return scope
	}
}
