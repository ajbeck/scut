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
		path := projectPathFor(t)

		// Install a couple of entries.
		install := &installCmd{Scope: "project", Only: []string{"post-tool-use", "status-line"}}
		if err := install.Run(io.Discard, fs, slog.Default()); err != nil {
			t.Fatalf("install: %v", err)
		}

		var stdout bytes.Buffer
		cmd := &statusCmd{Scope: "project"}
		if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}

		out := stdout.String()
		if !bytes.Contains(stdout.Bytes(), []byte("scut claude status-line")) {
			t.Errorf("status-line entry missing in human output\npath: %q\noutput:\n%s", path, out)
		}
		if !bytes.Contains(stdout.Bytes(), []byte("scut claude hook post-tool-use")) {
			t.Errorf("post-tool-use entry missing in human output\npath: %q\noutput:\n%s", path, out)
		}
	})

	t.Run("status_human_output_reports_missing_file", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())

		var stdout bytes.Buffer
		cmd := &statusCmd{Scope: "project"}
		if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}

		out := stdout.String()
		// Should mention the file doesn't exist or show "(no scut entries)" / "(file does not exist)".
		if !bytes.Contains(stdout.Bytes(), []byte("(file does not exist)")) &&
			!bytes.Contains(stdout.Bytes(), []byte("(no scut entries)")) {
			t.Errorf("expected missing-file message in human output, got:\n%s", out)
		}
	})

	t.Run("status_json_emits_structured_object", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		t.Chdir(t.TempDir())

		// Install a minimal set.
		install := &installCmd{Scope: "project", Only: []string{"post-tool-use", "status-line"}}
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
			t.Fatalf("output is not valid JSON: %v\n%s", err, stdout.String())
		}
		if len(out.Scopes) != 1 {
			t.Fatalf("expected 1 scope, got %d", len(out.Scopes))
		}
		sr := out.Scopes[0]
		if sr.Scope != "project" {
			t.Errorf("scope = %q, want %q", sr.Scope, "project")
		}
		if !sr.Exists {
			t.Errorf("expected exists=true, got false")
		}
		if len(sr.Entries) < 2 {
			t.Errorf("expected at least 2 entries (statusLine + PostToolUse), got %d\n%s",
				len(sr.Entries), stdout.String())
		}

		// Verify statusLine entry.
		var foundStatusLine, foundHook bool
		for _, e := range sr.Entries {
			if e.Kind == "statusLine" {
				foundStatusLine = true
			}
			if e.Kind == "hook" && e.Event == "PostToolUse" {
				foundHook = true
				if e.Matcher != "Write|Edit" {
					t.Errorf("PostToolUse matcher = %q, want %q", e.Matcher, "Write|Edit")
				}
			}
		}
		if !foundStatusLine {
			t.Errorf("statusLine entry missing from JSON output\n%s", stdout.String())
		}
		if !foundHook {
			t.Errorf("PostToolUse hook entry missing from JSON output\n%s", stdout.String())
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
			t.Fatalf("output is not valid JSON: %v\n%s", err, stdout.String())
		}
		if len(out.Scopes) != 2 {
			t.Fatalf("expected 2 scopes (project + user), got %d", len(out.Scopes))
		}

		scopeNames := make(map[string]bool)
		for _, s := range out.Scopes {
			scopeNames[s.Scope] = true
		}
		if !scopeNames["project"] {
			t.Errorf("expected project scope in output, got scopes: %v", scopeNames)
		}
		if !scopeNames["user"] {
			t.Errorf("expected user scope in output, got scopes: %v", scopeNames)
		}
	})
}
