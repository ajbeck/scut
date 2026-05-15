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
		err := cmd.Run(io.Discard, &stderr, fs, slog.Default())
		if err != nil {
			t.Fatalf("Run() expected nil error for missing file, got: %v", err)
		}
		if stderr.Len() == 0 {
			t.Error("expected message on stderr for missing file, got nothing")
		}
	})

	t.Run("uninstall_after_install_clears_all_scut_entries", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())
		path := projectPathFor(t)

		// First install everything.
		install := &installCmd{Scope: "project"}
		if err := install.Run(io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("install Run() error: %v", err)
		}

		// Now uninstall everything.
		uninstall := &uninstallCmd{Scope: "project"}
		if err := uninstall.Run(io.Discard, io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("uninstall Run() error: %v", err)
		}

		data, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}

		// Neither statusLine nor hooks should be present.
		if bytes.Contains(data, []byte(`"statusLine"`)) {
			t.Errorf("statusLine still present after uninstall\n%s", data)
		}
		if bytes.Contains(data, []byte(`"hooks"`)) {
			t.Errorf("hooks still present after uninstall\n%s", data)
		}
	})

	t.Run("uninstall_preserves_foreign_top_level_keys", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())
		path := projectPathFor(t)

		// Seed with a scut install + foreign keys.
		install := &installCmd{Scope: "project"}
		if err := install.Run(io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("install: %v", err)
		}
		// Add foreign keys to the file by re-seeding.
		seedJSON(t, fs, path, `{"allowedTools":["bash"],"statusLine":{"type":"command","command":"scut claude status-line"}}`)

		uninstall := &uninstallCmd{Scope: "project"}
		if err := uninstall.Run(io.Discard, io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("uninstall Run() error: %v", err)
		}

		data, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if !bytes.Contains(data, []byte(`"allowedTools"`)) {
			t.Errorf("foreign key %q missing after uninstall\n%s", "allowedTools", data)
		}
	})

	t.Run("uninstall_only_status_line_leaves_hooks_in_place", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())
		path := projectPathFor(t)

		// Install everything, then uninstall only status-line.
		install := &installCmd{Scope: "project"}
		if err := install.Run(io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("install: %v", err)
		}

		uninstall := &uninstallCmd{Scope: "project", Only: []string{"status-line"}}
		if err := uninstall.Run(io.Discard, io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("uninstall Run() error: %v", err)
		}

		data, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if bytes.Contains(data, []byte(`"statusLine"`)) {
			t.Errorf("statusLine still present after --only=status-line uninstall\n%s", data)
		}
		if !bytes.Contains(data, []byte(`"hooks"`)) {
			t.Errorf("hooks should remain when only status-line is uninstalled\n%s", data)
		}
	})

	t.Run("uninstall_drops_scut_group_keeps_foreign_group", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())
		path := projectPathFor(t)

		// Seed with a mix: scut group + foreign group for PostToolUse.
		seedJSON(t, fs, path, `{
  "hooks": {
    "PostToolUse": [
      {"matcher":"Write|Edit","hooks":[{"type":"command","command":"scut claude hook post-tool-use","statusMessage":"Formatting..."}]},
      {"matcher":"*","hooks":[{"type":"command","command":"other-formatter --check"}]}
    ]
  }
}`)

		uninstall := &uninstallCmd{Scope: "project", Only: []string{"post-tool-use"}}
		if err := uninstall.Run(io.Discard, io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("uninstall Run() error: %v", err)
		}

		data, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}

		// Botctrl group should be gone.
		if bytes.Contains(data, []byte("scut claude hook post-tool-use")) {
			t.Errorf("scut hook still present after uninstall\n%s", data)
		}
		// Foreign group should remain.
		if !bytes.Contains(data, []byte("other-formatter --check")) {
			t.Errorf("foreign hook missing after uninstall\n%s", data)
		}
	})

	t.Run("uninstall_dry_run_does_not_write_file", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())
		path := projectPathFor(t)

		// Install first.
		install := &installCmd{Scope: "project"}
		if err := install.Run(io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("install: %v", err)
		}
		original, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("reading original: %v", err)
		}

		// Dry-run uninstall.
		var stdout bytes.Buffer
		uninstall := &uninstallCmd{Scope: "project", DryRun: true}
		if err := uninstall.Run(&stdout, io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("uninstall Run() error: %v", err)
		}

		// File must be unchanged.
		after, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("reading after dry-run: %v", err)
		}
		if !bytes.Equal(original, after) {
			t.Errorf("file changed after --dry-run uninstall\nbefore:\n%s\nafter:\n%s", original, after)
		}
		if stdout.Len() == 0 {
			t.Error("expected stdout output from --dry-run uninstall, got nothing")
		}
	})
}
