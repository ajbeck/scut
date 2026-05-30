package initcmd

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"
)

func TestRunDryRunAllPrintsCompactPlan(t *testing.T) {
	fs := afero.NewMemMapFs()
	t.Chdir(t.TempDir())

	var stdout bytes.Buffer
	cmd := &Cmd{All: true, Scope: "project", DryRun: true}
	if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	out := stdout.String()
	if !bytes.Contains(stdout.Bytes(), []byte("CLAUDE project")) {
		t.Errorf("Claude dry-run header missing\n%s", out)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("CODEX")) || !bytes.Contains(stdout.Bytes(), []byte("project")) {
		t.Errorf("Codex dry-run header missing\n%s", out)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("would update")) {
		t.Errorf("dry-run summary missing\n%s", out)
	}
	if bytes.Contains(stdout.Bytes(), []byte("scut claude hook post-tool-use")) {
		t.Errorf("compact dry-run should not print full Claude JSON\n%s", out)
	}
}

func TestRunDryRunVerbosePrintsRenderedConfig(t *testing.T) {
	fs := afero.NewMemMapFs()
	t.Chdir(t.TempDir())

	var stdout bytes.Buffer
	cmd := &Cmd{All: true, Scope: "project", DryRun: true, Verbose: true}
	if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	out := stdout.String()
	if !bytes.Contains(stdout.Bytes(), []byte("scut claude hook post-tool-use")) {
		t.Errorf("Claude config output missing\n%s", out)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("scut codex hook post-tool-use")) {
		t.Errorf("Codex config output missing\n%s", out)
	}
}

func TestRunDryRunAllPassesLoggingFlags(t *testing.T) {
	fs := afero.NewMemMapFs()
	t.Chdir(t.TempDir())

	var stdout bytes.Buffer
	cmd := &Cmd{All: true, Scope: "project", BakeLogLevel: "debug", DryRun: true, Verbose: true}
	if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !bytes.Contains(stdout.Bytes(), []byte("scut claude --log-level=debug hook post-tool-use")) {
		t.Errorf("Claude baked log level missing\n%s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("scut codex --log-level=debug hook post-tool-use")) {
		t.Errorf("Codex baked log level missing\n%s", stdout.String())
	}
}

func TestRunExplicitCodexWritesOnlyCodexConfig(t *testing.T) {
	fs := afero.NewMemMapFs()
	t.Chdir(t.TempDir())

	var stdout bytes.Buffer
	cmd := &Cmd{Codex: true, Scope: "project"}
	if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if _, err := fs.Stat(filepath.Join(cwd, ".codex", "hooks.json")); err != nil {
		t.Fatalf("expected .codex/hooks.json to be written: %v", err)
	}
	if _, err := fs.Stat(filepath.Join(cwd, ".claude", "settings.json")); err == nil {
		t.Fatal("did not expect .claude/settings.json to be written")
	}
	if !bytes.Contains(stdout.Bytes(), []byte("CODEX")) || !bytes.Contains(stdout.Bytes(), []byte("project")) {
		t.Errorf("Codex install output missing\n%s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("scut doctor --scope=project")) {
		t.Errorf("post-install doctor guidance missing\n%s", stdout.String())
	}
}

func TestRunPreflightsAllTargetsBeforeWriting(t *testing.T) {
	fs := afero.NewMemMapFs()
	t.Chdir(t.TempDir())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	blockedPath := filepath.Join(cwd, ".codex", "hooks.json")
	if err := fs.MkdirAll(blockedPath, 0755); err != nil {
		t.Fatalf("mkdir blocked target: %v", err)
	}

	var stdout bytes.Buffer
	cmd := &Cmd{All: true, Scope: "project"}
	err = cmd.Run(&stdout, fs, slog.Default())
	if err == nil {
		t.Fatal("Run() expected preflight error")
	}
	if _, statErr := fs.Stat(filepath.Join(cwd, ".claude", "settings.json")); statErr == nil {
		t.Fatal("Claude settings were written despite Codex preflight failure")
	}
}

func TestRunNoDetectedAgentsReportsSkipped(t *testing.T) {
	fs := afero.NewMemMapFs()
	t.Chdir(t.TempDir())

	var stdout bytes.Buffer
	cmd := &Cmd{Scope: "project"}
	if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !bytes.Contains(stdout.Bytes(), []byte("no supported agents detected")) {
		t.Errorf("expected no-detected message\n%s", stdout.String())
	}
}

func TestRunUserScopeUsesPathDetection(t *testing.T) {
	fs := afero.NewMemMapFs()
	t.Chdir(t.TempDir())
	oldLookPath := lookPath
	lookPath = func(binary string) (string, error) {
		if binary == "codex" {
			return "/usr/local/bin/codex", nil
		}
		return "", errors.New("not found")
	}
	t.Cleanup(func() {
		lookPath = oldLookPath
	})

	var stdout bytes.Buffer
	cmd := &Cmd{Scope: "user", DryRun: true}
	if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if bytes.Contains(stdout.Bytes(), []byte("CLAUDE user")) {
		t.Errorf("did not expect Claude user config output\n%s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("CODEX")) || !bytes.Contains(stdout.Bytes(), []byte("user")) {
		t.Errorf("expected Codex user config output\n%s", stdout.String())
	}
}

func TestKongWiring(t *testing.T) {
	var cli struct {
		Init Cmd `cmd:""`
	}
	parser := kong.Must(&cli, kong.Name("scut"))
	if _, err := parser.Parse([]string{"init", "--all", "--scope=project", "--dry-run", "--verbose"}); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
}
