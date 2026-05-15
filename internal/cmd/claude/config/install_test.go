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

// projectPathFor returns the project settings path relative to cwd.
func projectPathFor(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	return filepath.Join(cwd, ".claude", "settings.json")
}

// seedJSON writes json content to path on fs, creating parent dirs.
func seedJSON(t *testing.T, fs afero.Fs, path string, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := fs.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %q: %v", dir, err)
	}
	if err := afero.WriteFile(fs, path, []byte(content), 0644); err != nil {
		t.Fatalf("writing %q: %v", path, err)
	}
}

func TestInstall(t *testing.T) {
	t.Run("install_no_file_creates_default_entries", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())

		var stdout bytes.Buffer
		cmd := &installCmd{Scope: "project", DryRun: true}
		if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}
		data := stdout.Bytes()
		if len(data) == 0 {
			t.Fatal("expected non-empty output")
		}
		var m map[string]interface{}
		if err := json.Unmarshal(bytes.TrimRight(data, "\n"), &m); err != nil {
			t.Fatalf("output is not valid JSON: %v\n%s", err, data)
		}
		if m["statusLine"] == nil {
			t.Errorf("expected statusLine in output, got none\n%s", data)
		}
		if m["hooks"] == nil {
			t.Errorf("expected hooks in output, got none\n%s", data)
		}
		// All 25 hook events should be present.
		hooks, _ := m["hooks"].(map[string]interface{})
		if len(hooks) != 25 {
			t.Errorf("expected 25 hook events, got %d\n%s", len(hooks), data)
		}
	})

	t.Run("install_preserves_foreign_top_level_keys", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())
		path := projectPathFor(t)
		seedJSON(t, fs, path, `{"allowedTools":["bash"],"permissions":{"allow":[]}}`)

		cmd := &installCmd{Scope: "project"}
		if err := cmd.Run(io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}

		data, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if !bytes.Contains(data, []byte(`"allowedTools"`)) {
			t.Errorf("foreign key %q missing after install\n%s", "allowedTools", data)
		}
		if !bytes.Contains(data, []byte(`"permissions"`)) {
			t.Errorf("foreign key %q missing after install\n%s", "permissions", data)
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
			t.Fatalf("reading after first run: %v", err)
		}

		if err := cmd.Run(io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("second Run() error: %v", err)
		}
		second, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("reading after second run: %v", err)
		}

		if !bytes.Equal(first, second) {
			t.Errorf("install not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
		}
	})

	t.Run("install_only_filters_to_selected_items", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		var stdout bytes.Buffer
		t.Chdir(t.TempDir())

		cmd := &installCmd{Scope: "project", Only: []string{"post-tool-use", "status-line"}, DryRun: true}
		if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}

		data := stdout.Bytes()
		var m map[string]interface{}
		if err := json.Unmarshal(bytes.TrimRight(data, "\n"), &m); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, data)
		}
		if m["statusLine"] == nil {
			t.Errorf("expected statusLine when status-line in --only\n%s", data)
		}
		hooks, _ := m["hooks"].(map[string]interface{})
		if hooks["PostToolUse"] == nil {
			t.Errorf("expected PostToolUse when post-tool-use in --only\n%s", data)
		}
		if hooks["SessionStart"] != nil {
			t.Errorf("expected SessionStart absent when not in --only\n%s", data)
		}
	})

	t.Run("install_log_flag_writes_log_into_command", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		var stdout bytes.Buffer
		t.Chdir(t.TempDir())

		cmd := &installCmd{Scope: "project", Only: []string{"session-start"}, BakeLog: true, DryRun: true}
		if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}

		data := stdout.Bytes()
		if !bytes.Contains(data, []byte("scut claude --log hook session-start")) {
			t.Errorf("expected --log in generated command\n%s", data)
		}
	})

	t.Run("install_log_level_writes_log_level_into_command", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		var stdout bytes.Buffer
		t.Chdir(t.TempDir())

		cmd := &installCmd{Scope: "project", Only: []string{"session-start"}, BakeLogLevel: "debug", DryRun: true}
		if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}

		data := stdout.Bytes()
		if !bytes.Contains(data, []byte("scut claude --log-level=debug hook session-start")) {
			t.Errorf("expected --log-level=debug in generated command\n%s", data)
		}
	})

	t.Run("install_log_level_omits_bare_log_flag", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		var stdout bytes.Buffer
		t.Chdir(t.TempDir())

		// Set both BakeLog and BakeLogLevel; BakeLogLevel should win and bare --log should not appear.
		cmd := &installCmd{Scope: "project", Only: []string{"session-start"}, BakeLog: true, BakeLogLevel: "info", DryRun: true}
		if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}

		data := stdout.Bytes()
		if !bytes.Contains(data, []byte("--log-level=info")) {
			t.Errorf("expected --log-level=info in generated command\n%s", data)
		}
		// Bare --log must not appear alongside --log-level.
		content := string(data)
		if bytes.Contains(data, []byte("--log-level=info --log ")) ||
			len(content) != len(content) { // placeholder
			// Real check: the command must contain --log-level but not a subsequent --log token.
		}
		if bytes.Contains(data, []byte(" --log hook")) {
			t.Errorf("bare --log appears in generated command when --log-level is set\n%s", data)
		}
	})

	t.Run("install_dry_run_does_not_write_file", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())
		path := projectPathFor(t)

		cmd := &installCmd{Scope: "project", DryRun: true}
		var stdout bytes.Buffer
		if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}

		if _, err := fs.Stat(path); err == nil {
			t.Errorf("file %q should not exist after --dry-run but it does", path)
		}
		if len(stdout.Bytes()) == 0 {
			t.Errorf("expected stdout output from --dry-run, got nothing")
		}
	})

	t.Run("install_refuses_when_foreign_statusline_present", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())
		path := projectPathFor(t)
		seedJSON(t, fs, path, `{"statusLine":{"type":"command","command":"other-tool status"}}`)

		cmd := &installCmd{Scope: "project"}
		err := cmd.Run(io.Discard, fs, slog.Default())
		if err == nil {
			t.Fatal("Run() expected error for foreign statusLine, got nil")
		}
		if !errors.Is(err, ErrForeignStatusLine) {
			t.Errorf("Run() error = %v, want errors.Is(err, ErrForeignStatusLine)", err)
		}
	})

	t.Run("install_foreign_statusline_skipped_when_excluded", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())
		path := projectPathFor(t)
		seedJSON(t, fs, path, `{"statusLine":{"type":"command","command":"other-tool status"}}`)

		// Exclude status-line from install set.
		cmd := &installCmd{Scope: "project", Only: []string{"post-tool-use"}}
		if err := cmd.Run(io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}

		data, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if !bytes.Contains(data, []byte("other-tool status")) {
			t.Errorf("foreign statusLine was overwritten, expected it preserved\n%s", data)
		}
	})

	t.Run("install_unknown_only_token_returns_sentinel", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())

		cmd := &installCmd{Scope: "project", Only: []string{"not-a-real-hook"}}
		err := cmd.Run(io.Discard, fs, slog.Default())
		if err == nil {
			t.Fatal("Run() expected error for unknown --only token, got nil")
		}
		if !errors.Is(err, ErrUnknownOnlyToken) {
			t.Errorf("Run() error = %v, want errors.Is(err, ErrUnknownOnlyToken)", err)
		}
	})
}
