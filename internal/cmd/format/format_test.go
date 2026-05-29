package format

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	byteformat "github.com/ajbeck/scut/internal/format"
)

func TestFormatFilesHonorsIgnoreFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("Mkdir(.git): %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("Mkdir(docs): %v", err)
	}
	ignoredPath := filepath.Join(dir, "docs", "template.md")
	if err := os.WriteFile(filepath.Join(dir, ".prettierignore"), []byte("docs/\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.prettierignore): %v", err)
	}
	if err := os.WriteFile(ignoredPath, []byte("#  Hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(template.md): %v", err)
	}

	var stdout bytes.Buffer
	if err := formatFiles(&stdout, []string{ignoredPath}, byteformat.FormatMarkdown, false); err != nil {
		t.Fatalf("formatFiles() error = %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Errorf("formatFiles() output = %q, want empty output for ignored file", got)
	}
}

func TestFormatFilesForceBypassesIgnoreFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("Mkdir(.git): %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("Mkdir(docs): %v", err)
	}
	ignoredPath := filepath.Join(dir, "docs", "template.md")
	if err := os.WriteFile(filepath.Join(dir, ".scutignore"), []byte("docs/\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.scutignore): %v", err)
	}
	if err := os.WriteFile(ignoredPath, []byte("#  Hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(template.md): %v", err)
	}

	var stdout bytes.Buffer
	if err := formatFiles(&stdout, []string{ignoredPath}, byteformat.FormatMarkdown, true); err != nil {
		t.Fatalf("formatFiles() error = %v", err)
	}
	if got, want := stdout.String(), "# Hello\n"; got != want {
		t.Errorf("formatFiles() output = %q, want %q", got, want)
	}
}
