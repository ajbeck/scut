package logging

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanCmd_RemovesOldFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	logDir := filepath.Join(dir, dirName)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create an old file and a recent file.
	oldFile := filepath.Join(logDir, "20260101_old-hook.jsonl")
	newFile := filepath.Join(logDir, "20260403_new-hook.jsonl")

	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	// Backdate the old file.
	old := time.Now().AddDate(0, 0, -30)
	if err := os.Chtimes(oldFile, old, old); err != nil {
		t.Fatal(err)
	}

	cmd := &cleanCmd{Days: 7}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old file should have been removed")
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Error("new file should still exist")
	}
}

func TestCleanCmd_AllFlag(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	logDir := filepath.Join(dir, dirName)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := []string{
		"20260403_hook-a.jsonl",
		"20260403_hook-b.jsonl",
		"20260402_hook-a.jsonl.1712345678",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(logDir, f), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cmd := &cleanCmd{All: true}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	entries, _ := os.ReadDir(logDir)
	if len(entries) != 0 {
		t.Errorf("expected empty directory, got %d entries", len(entries))
	}
}

func TestCleanCmd_SkipsNonLogFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	logDir := filepath.Join(dir, dirName)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a non-log file.
	other := filepath.Join(logDir, "notes.txt")
	if err := os.WriteFile(other, []byte("keep me"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &cleanCmd{All: true}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if _, err := os.Stat(other); err != nil {
		t.Error("non-log file should not have been removed")
	}
}

func TestIsLogFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"20260403_post-tool-use.jsonl", true},
		{"20260403_hook.jsonl.1712345678", true},
		{"notes.txt", false},
		{"readme.md", false},
		{".jsonl", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLogFile(tt.name); got != tt.want {
				t.Errorf("isLogFile(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
