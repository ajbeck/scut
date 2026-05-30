//go:build goexperiment.jsonv2

package config

import (
	"bytes"
	"io"
	"log/slog"
	"testing"

	json "encoding/json/v2"

	"github.com/spf13/afero"
)

func TestStatus(t *testing.T) {
	t.Run("status_human_output_lists_entries", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())

		install := &installCmd{Scope: "project"}
		if err := install.Run(io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("install: %v", err)
		}

		var stdout bytes.Buffer
		cmd := &statusCmd{Scope: "project"}
		if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}
		if !bytes.Contains(stdout.Bytes(), []byte("scut codex hook post-tool-use")) {
			t.Errorf("post-tool-use entry missing\n%s", stdout.String())
		}
	})

	t.Run("status_json_emits_structured_object", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())

		install := &installCmd{Scope: "project"}
		if err := install.Run(io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("install: %v", err)
		}

		var stdout bytes.Buffer
		cmd := &statusCmd{Scope: "project", JSON: true}
		if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}

		var out statusOutput
		if err := json.Unmarshal(bytes.TrimRight(stdout.Bytes(), "\n"), &out); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
		}
		if len(out.Scopes) != 1 {
			t.Fatalf("expected one scope, got %d", len(out.Scopes))
		}
		if len(out.Scopes[0].Entries) != 1 {
			t.Fatalf("expected one entry, got %d\n%s", len(out.Scopes[0].Entries), stdout.String())
		}
		entry := out.Scopes[0].Entries[0]
		if entry.Event != "PostToolUse" {
			t.Errorf("event = %q", entry.Event)
		}
		if entry.Matcher != "apply_patch|Edit|Write" {
			t.Errorf("matcher = %q", entry.Matcher)
		}
	})

	t.Run("status_scope_both_inspects_user_and_project", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())

		var stdout bytes.Buffer
		cmd := &statusCmd{Scope: "both", JSON: true}
		if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}

		var out statusOutput
		if err := json.Unmarshal(bytes.TrimRight(stdout.Bytes(), "\n"), &out); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
		}
		if len(out.Scopes) != 2 {
			t.Fatalf("expected project + user scopes, got %d", len(out.Scopes))
		}
	})
}
