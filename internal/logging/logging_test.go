package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpen_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	logger, closer, err := Open("test-hook", slog.LevelWarn)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer closer.Close()

	logger.Warn("hello", "key", "value")

	path := filepath.Join(dir, dirName, fileName("test-hook"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if !strings.Contains(string(data), `"msg":"hello"`) {
		t.Errorf("log file missing expected message, got: %s", data)
	}
	if !strings.Contains(string(data), `"key":"value"`) {
		t.Errorf("log file missing expected attribute, got: %s", data)
	}
}

func TestOpen_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	logDir := filepath.Join(dir, dirName)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(logDir, fileName("append-test"))
	if err := os.WriteFile(path, []byte(`{"existing":"line"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	logger, closer, err := Open("append-test", slog.LevelInfo)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer closer.Close()

	logger.Info("second")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("Open() should append; got %d lines, want 2", len(lines))
	}
}

func TestOpen_RotatesLargeFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	logDir := filepath.Join(dir, dirName)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(logDir, fileName("rotate-test"))
	large := make([]byte, maxFileSize+1)
	for i := range large {
		large[i] = 'x'
	}
	if err := os.WriteFile(path, large, 0644); err != nil {
		t.Fatal(err)
	}

	_, closer, err := Open("rotate-test", slog.LevelWarn)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	closer.Close()

	// The original file should be fresh (small).
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("new log file missing: %v", err)
	}
	if info.Size() > maxFileSize {
		t.Errorf("new file size = %d, should be small after rotation", info.Size())
	}

	// A rotated file should exist.
	entries, _ := os.ReadDir(logDir)
	rotated := 0
	for _, e := range entries {
		if strings.Contains(e.Name(), ".jsonl.") {
			rotated++
		}
	}
	if rotated != 1 {
		t.Errorf("rotated file count = %d, want 1", rotated)
	}
}

func TestOpen_LevelFiltering(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	logger, closer, err := Open("level-test", slog.LevelWarn)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer closer.Close()

	logger.Info("should be filtered")
	logger.Warn("should appear")

	path := filepath.Join(dir, dirName, fileName("level-test"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, "should be filtered") {
		t.Error("info message should be filtered at warn level")
	}
	if !strings.Contains(content, "should appear") {
		t.Error("warn message should appear at warn level")
	}
}

func TestFileName(t *testing.T) {
	name := fileName("post-tool-use")
	today := time.Now().Format("20060102")
	want := today + "_post-tool-use.jsonl"
	if name != want {
		t.Errorf("fileName() = %q, want %q", name, want)
	}
}

func TestDiscard(t *testing.T) {
	// Discard logger should not panic when used.
	Discard.Info("this goes nowhere")
	Discard.Warn("this too", "key", "value")
}
