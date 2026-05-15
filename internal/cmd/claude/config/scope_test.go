//go:build goexperiment.jsonv2

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScopePaths(t *testing.T) {
	t.Run("project_settings_path_joins_cwd", func(t *testing.T) {
		cwd := "/home/user/myproject"
		got := projectSettingsPath(cwd)
		want := "/home/user/myproject/.claude/settings.json"
		if got != want {
			t.Errorf("projectSettingsPath(%q) = %q, want %q", cwd, got, want)
		}
	})

	t.Run("project_settings_path_handles_trailing_slash", func(t *testing.T) {
		cwd := "/home/user/myproject/"
		got := projectSettingsPath(cwd)
		// filepath.Join cleans trailing slashes.
		want := filepath.Join("/home/user/myproject", ".claude", "settings.json")
		if got != want {
			t.Errorf("projectSettingsPath(%q) = %q, want %q", cwd, got, want)
		}
		// Must not contain double slashes.
		if strings.Contains(got, "//") {
			t.Errorf("projectSettingsPath(%q) contains double slash: %q", cwd, got)
		}
	})

	t.Run("user_settings_path_uses_home_dir", func(t *testing.T) {
		got, err := userSettingsPath()
		if err != nil {
			t.Fatalf("userSettingsPath() error = %v", err)
		}
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("os.UserHomeDir() error = %v", err)
		}
		want := filepath.Join(home, ".claude", "settings.json")
		if got != want {
			t.Errorf("userSettingsPath() = %q, want %q", got, want)
		}
	})
}
