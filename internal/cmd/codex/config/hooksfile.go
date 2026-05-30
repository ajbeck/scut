//go:build goexperiment.jsonv2

package config

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"
)

// HooksFile is the subset of Codex hooks.json that scut manipulates.
// Foreign top-level keys round-trip through the Foreign field.
type HooksFile struct {
	Hooks   map[string][]HookGroup    `json:"hooks,omitzero"`
	Foreign map[string]jsontext.Value `json:",inline"`
}

// HookGroup is one matcher group for a Codex hook event.
type HookGroup struct {
	Matcher string                    `json:"matcher,omitzero"`
	Hooks   []HookEntry               `json:"hooks"`
	Foreign map[string]jsontext.Value `json:",inline"`
}

// HookEntry is one command hook handler.
type HookEntry struct {
	Type           string                    `json:"type"`
	Command        string                    `json:"command"`
	CommandWindows string                    `json:"commandWindows,omitzero"`
	Timeout        int                       `json:"timeout,omitzero"`
	StatusMessage  string                    `json:"statusMessage,omitzero"`
	Foreign        map[string]jsontext.Value `json:",inline"`
}

func marshalHooksFile(h HooksFile) ([]byte, error) {
	data, err := json.Marshal(h,
		json.Deterministic(true),
		jsontext.WithIndent("  "),
	)
	if err != nil {
		return nil, fmt.Errorf("marshalling hooks.json: %w", err)
	}
	return append(data, '\n'), nil
}

func readHooksFile(fs afero.Fs, path string) (HooksFile, error) {
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		if isNotExist(fs, path) {
			return HooksFile{}, nil
		}
		return HooksFile{}, fmt.Errorf("reading %q: %w", path, err)
	}
	var h HooksFile
	if err := json.Unmarshal(data, &h); err != nil {
		return HooksFile{}, fmt.Errorf("parsing %q: %w", path, err)
	}
	return h, nil
}

func writeHooksFile(fs afero.Fs, path string, h HooksFile) error {
	if err := fs.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating parent directory for %q: %w", path, err)
	}
	data, err := marshalHooksFile(h)
	if err != nil {
		return err
	}
	if err := afero.WriteFile(fs, path, data, 0644); err != nil {
		return fmt.Errorf("writing %q: %w", path, err)
	}
	return nil
}

func isNotExist(fs afero.Fs, path string) bool {
	_, err := fs.Stat(path)
	return err != nil
}
