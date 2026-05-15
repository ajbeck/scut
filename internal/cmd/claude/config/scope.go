//go:build goexperiment.jsonv2

package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// projectSettingsPath returns the project-scope settings path relative to cwd.
func projectSettingsPath(cwd string) string {
	return filepath.Join(cwd, ".claude", "settings.json")
}

// userSettingsPath returns the user-scope settings path. It returns an error
// only when os.UserHomeDir fails.
func userSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving user home: %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}
