//go:build goexperiment.jsonv2

package config

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	json "encoding/json/v2"

	"github.com/spf13/afero"
)

func projectPathFor(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	return filepath.Join(cwd, ".codex", "hooks.json")
}

func seedJSON(t *testing.T, fs afero.Fs, path string, content string) {
	t.Helper()
	if err := fs.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %q: %v", filepath.Dir(path), err)
	}
	if err := afero.WriteFile(fs, path, []byte(content), 0644); err != nil {
		t.Fatalf("writing %q: %v", path, err)
	}
}

func TestInstall(t *testing.T) {
	t.Run("install_no_file_defaults_to_post_tool_use", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())

		var stdout bytes.Buffer
		cmd := &installCmd{Scope: "project", DryRun: true}
		if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}

		var out HooksFile
		if err := json.Unmarshal(bytes.TrimRight(stdout.Bytes(), "\n"), &out); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
		}
		if len(out.Hooks) != 1 {
			t.Fatalf("expected one default hook event, got %d\n%s", len(out.Hooks), stdout.String())
		}
		groups := out.Hooks["PostToolUse"]
		if len(groups) != 1 {
			t.Fatalf("expected PostToolUse group, got %#v", out.Hooks)
		}
		if groups[0].Matcher != "apply_patch|Edit|Write" {
			t.Errorf("matcher = %q, want %q", groups[0].Matcher, "apply_patch|Edit|Write")
		}
		if got := groups[0].Hooks[0].Command; got != "scut codex hook post-tool-use" {
			t.Errorf("command = %q", got)
		}
	})

	t.Run("install_preserves_foreign_top_level_keys_and_hooks", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())
		path := projectPathFor(t)
		seedJSON(t, fs, path, `{"metadata":{"owner":"user"},"hooks":{"Stop":[{"matcher":"*","hooks":[{"type":"command","command":"other stop"}]}]}}`)

		cmd := &installCmd{Scope: "project"}
		if err := cmd.Run(io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}

		data, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if !bytes.Contains(data, []byte(`"metadata"`)) {
			t.Errorf("foreign key missing after install\n%s", data)
		}
		if !bytes.Contains(data, []byte(`other stop`)) {
			t.Errorf("foreign hook missing after install\n%s", data)
		}
		if !bytes.Contains(data, []byte(`scut codex hook post-tool-use`)) {
			t.Errorf("scut hook missing after install\n%s", data)
		}
	})

	t.Run("install_idempotent_bytes_equal_on_second_run", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())
		path := projectPathFor(t)

		cmd := &installCmd{Scope: "project"}
		if err := cmd.Run(io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("first Run() error: %v", err)
		}
		first, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("reading first file: %v", err)
		}
		if err := cmd.Run(io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("second Run() error: %v", err)
		}
		second, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("reading second file: %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Errorf("install not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
		}
	})

	t.Run("install_only_wires_explicit_events", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())

		var stdout bytes.Buffer
		cmd := &installCmd{Scope: "project", Only: []string{"session-start", "post-tool-use"}, DryRun: true}
		if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}

		var out HooksFile
		if err := json.Unmarshal(bytes.TrimRight(stdout.Bytes(), "\n"), &out); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
		}
		if out.Hooks["SessionStart"] == nil {
			t.Errorf("SessionStart missing from --only output")
		}
		if out.Hooks["PostToolUse"] == nil {
			t.Errorf("PostToolUse missing from --only output")
		}
		if out.Hooks["Stop"] != nil {
			t.Errorf("Stop should not be installed by selected --only set")
		}
	})

	t.Run("install_log_level_writes_log_level_into_command", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())

		var stdout bytes.Buffer
		cmd := &installCmd{Scope: "project", BakeLogLevel: "debug", DryRun: true}
		if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}
		if !bytes.Contains(stdout.Bytes(), []byte("scut codex --log-level=debug hook post-tool-use")) {
			t.Errorf("expected --log-level=debug in generated command\n%s", stdout.String())
		}
	})

	t.Run("install_dry_run_does_not_write_file", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())
		path := projectPathFor(t)

		var stdout bytes.Buffer
		cmd := &installCmd{Scope: "project", DryRun: true}
		if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}
		if _, err := fs.Stat(path); err == nil {
			t.Errorf("file %q should not exist after --dry-run", path)
		}
		if stdout.Len() == 0 {
			t.Error("expected stdout output from --dry-run")
		}
	})

	t.Run("install_unknown_only_token_returns_sentinel", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())

		cmd := &installCmd{Scope: "project", Only: []string{"not-real"}}
		err := cmd.Run(io.Discard, fs, slog.Default())
		if err == nil {
			t.Fatal("Run() expected error for unknown --only token")
		}
		if !errors.Is(err, ErrUnknownOnlyToken) {
			t.Errorf("Run() error = %v, want errors.Is(err, ErrUnknownOnlyToken)", err)
		}
	})
}
