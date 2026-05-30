//go:build goexperiment.jsonv2

package config

import (
	"bytes"
	"io"
	"log/slog"
	"testing"

	"github.com/spf13/afero"
)

func TestUninstall(t *testing.T) {
	t.Run("uninstall_no_file_exits_zero_with_stderr", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())

		var stderr bytes.Buffer
		cmd := &uninstallCmd{Scope: "project"}
		if err := cmd.Run(io.Discard, &stderr, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}
		if stderr.Len() == 0 {
			t.Error("expected stderr message")
		}
	})

	t.Run("uninstall_after_default_install_removes_scut_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())
		path := projectPathFor(t)

		install := &installCmd{Scope: "project"}
		if err := install.Run(io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("install: %v", err)
		}
		uninstall := &uninstallCmd{Scope: "project"}
		if err := uninstall.Run(io.Discard, io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("uninstall: %v", err)
		}

		data, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if bytes.Contains(data, []byte(`"hooks"`)) {
			t.Errorf("hooks still present after uninstall\n%s", data)
		}
	})

	t.Run("uninstall_drops_scut_group_keeps_foreign_group", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())
		path := projectPathFor(t)
		seedJSON(t, fs, path, `{
  "hooks": {
    "PostToolUse": [
      {"matcher":"apply_patch|Edit|Write","hooks":[{"type":"command","command":"scut codex hook post-tool-use","statusMessage":"Formatting..."}]},
      {"matcher":"*","hooks":[{"type":"command","command":"other hook"}]}
    ]
  }
}`)

		cmd := &uninstallCmd{Scope: "project"}
		if err := cmd.Run(io.Discard, io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}

		data, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if bytes.Contains(data, []byte("scut codex hook post-tool-use")) {
			t.Errorf("scut hook still present after uninstall\n%s", data)
		}
		if !bytes.Contains(data, []byte("other hook")) {
			t.Errorf("foreign hook missing after uninstall\n%s", data)
		}
	})

	t.Run("uninstall_default_removes_explicitly_installed_extra_hooks", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())
		path := projectPathFor(t)

		install := &installCmd{Scope: "project", Only: []string{"session-start", "post-tool-use"}}
		if err := install.Run(io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("install: %v", err)
		}
		uninstall := &uninstallCmd{Scope: "project"}
		if err := uninstall.Run(io.Discard, io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("uninstall: %v", err)
		}

		data, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if bytes.Contains(data, []byte("scut codex hook session-start")) {
			t.Errorf("session-start still present after default uninstall\n%s", data)
		}
		if bytes.Contains(data, []byte("scut codex hook post-tool-use")) {
			t.Errorf("post-tool-use still present after default uninstall\n%s", data)
		}
	})
}
