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

func TestRunDryRunAllPrintsGroupedAgentOutput(t *testing.T) {
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
	if !bytes.Contains(stdout.Bytes(), []byte("CODEX project")) {
		t.Errorf("Codex dry-run header missing\n%s", out)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("scut claude hook post-tool-use")) {
		t.Errorf("Claude config output missing\n%s", out)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("scut codex hook post-tool-use")) {
		t.Errorf("Codex config output missing\n%s", out)
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
	if !bytes.Contains(stdout.Bytes(), []byte("CODEX project")) {
		t.Errorf("Codex install output missing\n%s", stdout.String())
	}
}

func TestRunNoDetectedAgentsReportsSkipped(t *testing.T) {
	fs := afero.NewMemMapFs()
	t.Chdir(t.TempDir())
	oldLookPath := lookPath
	lookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}
	t.Cleanup(func() {
		lookPath = oldLookPath
	})

	var stdout bytes.Buffer
	cmd := &Cmd{Scope: "project"}
	if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !bytes.Contains(stdout.Bytes(), []byte("no supported agents detected")) {
		t.Errorf("expected no-detected message\n%s", stdout.String())
	}
}

func TestKongWiring(t *testing.T) {
	var cli struct {
		Init Cmd `cmd:""`
	}
	parser := kong.Must(&cli, kong.Name("scut"))
	if _, err := parser.Parse([]string{"init", "--all", "--scope=project", "--dry-run"}); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
}
