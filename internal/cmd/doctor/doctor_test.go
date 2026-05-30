//go:build goexperiment.jsonv2

package doctor

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	json "encoding/json/v2"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"
)

func TestRunHumanReportsInstalledHooks(t *testing.T) {
	fs := afero.NewMemMapFs()
	t.Chdir(t.TempDir())
	oldLookPath := lookPath
	lookPath = func(binary string) (string, error) {
		if binary == "scut" {
			return "/usr/local/bin/scut", nil
		}
		return "", errors.New("not found")
	}
	t.Cleanup(func() { lookPath = oldLookPath })

	seedFile(t, fs, ".claude/settings.json", `{
  "statusLine": {"type": "command", "command": "scut claude status-line"},
  "hooks": {
    "PostToolUse": [
      {"matcher": "Write|Edit", "hooks": [{"type": "command", "command": "scut claude hook post-tool-use"}]}
    ]
  }
}`)
	seedFile(t, fs, ".codex/hooks.json", `{
  "hooks": {
    "PostToolUse": [
      {"matcher": "apply_patch|Edit|Write", "hooks": [{"type": "command", "command": "scut codex hook post-tool-use"}]}
    ]
  }
}`)

	var stdout bytes.Buffer
	cmd := &Cmd{Scope: "project"}
	if err := cmd.Run(&stdout, fs, slog.Default()); err != nil {
		t.Fatalf("Run() error: %v\n%s", err, stdout.String())
	}
	out := stdout.String()
	if !bytes.Contains(stdout.Bytes(), []byte("scut is discoverable on PATH")) {
		t.Errorf("missing PATH check\n%s", out)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("1 scut Claude hooks and 1 status line found")) {
		t.Errorf("missing Claude entries check\n%s", out)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("scut Codex hook entries found")) {
		t.Errorf("missing Codex entries check\n%s", out)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("approve/trust this project")) {
		t.Errorf("missing Codex project trust note\n%s", out)
	}
}

func TestRunJSONReportsCodexTomlProblems(t *testing.T) {
	fs := afero.NewMemMapFs()
	t.Chdir(t.TempDir())
	oldLookPath := lookPath
	lookPath = func(binary string) (string, error) {
		if binary == "scut" {
			return "/usr/local/bin/scut", nil
		}
		return "", errors.New("not found")
	}
	t.Cleanup(func() { lookPath = oldLookPath })

	seedFile(t, fs, ".codex/hooks.json", `{"hooks":{"PostToolUse":[{"hooks":[{"type":"command","command":"scut codex hook post-tool-use"}]}]}}`)
	seedFile(t, fs, ".codex/config.toml", `
hooks = false

[hooks]
`)

	var stdout bytes.Buffer
	cmd := &Cmd{Codex: true, Scope: "project", JSON: true}
	err := cmd.Run(&stdout, fs, slog.Default())
	if err == nil {
		t.Fatal("Run() expected error when hooks are disabled")
	}

	var out output
	if unmarshalErr := json.Unmarshal(bytes.TrimRight(stdout.Bytes(), "\n"), &out); unmarshalErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", unmarshalErr, stdout.String())
	}
	if !hasFinding(out.Findings, "codex", "inline-hooks", severityWarn) {
		t.Errorf("inline-hooks warning missing: %#v", out.Findings)
	}
	if !hasFinding(out.Findings, "codex", "hooks-feature", severityError) {
		t.Errorf("hooks-feature error missing: %#v", out.Findings)
	}
}

func TestRunMissingScutPathReturnsError(t *testing.T) {
	fs := afero.NewMemMapFs()
	t.Chdir(t.TempDir())
	oldLookPath := lookPath
	lookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}
	t.Cleanup(func() { lookPath = oldLookPath })

	var stdout bytes.Buffer
	cmd := &Cmd{Codex: true, Scope: "project"}
	if err := cmd.Run(&stdout, fs, slog.Default()); err == nil {
		t.Fatal("Run() expected error when scut is not on PATH")
	}
	if !bytes.Contains(stdout.Bytes(), []byte("scut is not discoverable on PATH")) {
		t.Errorf("missing PATH error\n%s", stdout.String())
	}
}

func TestKongWiring(t *testing.T) {
	var cli struct {
		Doctor Cmd `cmd:""`
	}
	parser := kong.Must(&cli, kong.Name("scut"))
	if _, err := parser.Parse([]string{"doctor", "--codex", "--scope=both", "--json"}); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
}

func seedFile(t *testing.T, fs afero.Fs, path, content string) {
	t.Helper()
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("os.Getwd: %v", err)
		}
		path = filepath.Join(cwd, path)
	}
	if err := fs.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %q: %v", filepath.Dir(path), err)
	}
	if err := afero.WriteFile(fs, path, []byte(content), 0644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

func hasFinding(findings []finding, agentName, check string, sev severity) bool {
	for _, f := range findings {
		if f.Agent == agentName && f.Check == check && f.Severity == sev {
			return true
		}
	}
	return false
}
