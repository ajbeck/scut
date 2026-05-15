//go:build goexperiment.jsonv2

package config

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"
)

// Settings is the subset of Claude Code's settings.json that botctrl manipulates.
// Foreign top-level keys round-trip through the Foreign field without modification.
type Settings struct {
	StatusLine *StatusLine               `json:"statusLine,omitzero"`
	Hooks      map[string][]HookGroup    `json:"hooks,omitzero"`
	Foreign    map[string]jsontext.Value `json:",inline"`
}

// StatusLine represents the settings.json statusLine object.
type StatusLine struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// HookGroup is one entry in a settings.json hook array
// (e.g. settings.json#/hooks/PostToolUse[0]).
type HookGroup struct {
	Matcher string      `json:"matcher,omitzero"`
	Hooks   []HookEntry `json:"hooks"`
}

// HookEntry is one element within a HookGroup's hooks array.
type HookEntry struct {
	Type          string `json:"type"`
	Command       string `json:"command"`
	StatusMessage string `json:"statusMessage,omitzero"`
}

// marshalSettings encodes s as deterministic, indented JSON followed by a trailing newline.
// Foreign keys are written in alphabetical order after statusLine and hooks.
// The Deterministic option sorts map keys (including the inlined Foreign map) alphabetically.
// Explicit struct fields (StatusLine, Hooks) appear in declaration order before foreign keys.
func marshalSettings(s Settings) ([]byte, error) {
	data, err := json.Marshal(s,
		json.Deterministic(true),
		jsontext.WithIndent("  "),
	)
	if err != nil {
		return nil, fmt.Errorf("marshalling settings: %w", err)
	}
	// Append trailing newline (POSIX text file convention).
	return append(data, '\n'), nil
}

// readSettings reads and unmarshals the settings file at path from fs.
// If the file does not exist, it returns an empty Settings{} with no error.
func readSettings(fs afero.Fs, path string) (Settings, error) {
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		if isNotExist(fs, path) {
			return Settings{}, nil
		}
		return Settings{}, fmt.Errorf("reading %q: %w", path, err)
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, fmt.Errorf("parsing %q: %w", path, err)
	}
	return s, nil
}

// writeSettings marshals s and writes it to path on fs, creating parent
// directories with mode 0755 if they do not exist. The file is written
// at mode 0644.
func writeSettings(fs afero.Fs, path string, s Settings) error {
	if err := fs.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating parent directory for %q: %w", path, err)
	}
	data, err := marshalSettings(s)
	if err != nil {
		return err
	}
	if err := afero.WriteFile(fs, path, data, 0644); err != nil {
		return fmt.Errorf("writing %q: %w", path, err)
	}
	return nil
}

// isNotExist reports whether the file at path does not exist on fs.
func isNotExist(fs afero.Fs, path string) bool {
	_, err := fs.Stat(path)
	return err != nil
}
