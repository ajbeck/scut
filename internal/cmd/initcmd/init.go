package initcmd

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/afero"

	claudeconfig "github.com/ajbeck/scut/internal/cmd/claude/config"
	codexconfig "github.com/ajbeck/scut/internal/cmd/codex/config"
)

// Cmd is the Kong command for "scut init".
type Cmd struct {
	Claude bool   `help:"Set up Claude Code hooks."`
	Codex  bool   `help:"Set up Codex hooks."`
	All    bool   `help:"Set up all supported agents, even if not detected."`
	Scope  string `help:"Configuration scope: project or user." default:"project" enum:"project,user"`
	DryRun bool   `help:"Print the planned config output without writing files." name:"dry-run"`
}

type agent string

const (
	agentClaude agent = "claude"
	agentCodex  agent = "codex"
)

var lookPath = exec.LookPath

// Run installs scut's default hook configuration for selected agents.
func (c *Cmd) Run(stdout io.Writer, fs afero.Fs, logger *slog.Logger) error {
	agents, skipped, err := c.resolveAgents(fs)
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		for _, line := range skipped {
			fmt.Fprintln(stdout, line)
		}
		fmt.Fprintln(stdout, "no supported agents detected; pass --all, --claude, or --codex to select agents explicitly")
		return nil
	}

	for _, line := range skipped {
		fmt.Fprintln(stdout, line)
	}
	for _, a := range agents {
		if err := c.installAgent(stdout, fs, logger, a); err != nil {
			return err
		}
	}
	return nil
}

func (c *Cmd) resolveAgents(fs afero.Fs) ([]agent, []string, error) {
	if c.All {
		return []agent{agentClaude, agentCodex}, nil, nil
	}
	if c.Claude || c.Codex {
		var selected []agent
		if c.Claude {
			selected = append(selected, agentClaude)
		}
		if c.Codex {
			selected = append(selected, agentCodex)
		}
		return selected, nil, nil
	}

	var selected []agent
	var skipped []string
	if detectedAgent(fs, c.Scope, ".claude", "claude") {
		selected = append(selected, agentClaude)
	} else {
		skipped = append(skipped, "CLAUDE skipped (not detected)")
	}
	if detectedAgent(fs, c.Scope, ".codex", "codex") {
		selected = append(selected, agentCodex)
	} else {
		skipped = append(skipped, "CODEX  skipped (not detected)")
	}
	return selected, skipped, nil
}

func detectedAgent(fs afero.Fs, scope, configDir, binary string) bool {
	switch scope {
	case "project":
		if exists(fs, configDir) {
			return true
		}
	case "user":
		path, err := homeConfigDir(configDir)
		if err == nil && exists(fs, path) {
			return true
		}
	}
	_, err := lookPath(binary)
	return err == nil
}

func homeConfigDir(configDir string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir), nil
}

func exists(fs afero.Fs, path string) bool {
	_, err := fs.Stat(path)
	return err == nil
}

func (c *Cmd) installAgent(stdout io.Writer, fs afero.Fs, logger *slog.Logger, a agent) error {
	switch a {
	case agentClaude:
		path, err := claudeconfig.PathForScope(c.Scope)
		if err != nil {
			return err
		}
		return c.writeAgent(stdout, "CLAUDE", c.Scope, path, func(w io.Writer) error {
			return claudeconfig.Install(w, fs, logger, claudeconfig.InstallOptions{
				Scope:  c.Scope,
				DryRun: c.DryRun,
			})
		})
	case agentCodex:
		path, err := codexconfig.PathForScope(c.Scope)
		if err != nil {
			return err
		}
		return c.writeAgent(stdout, "CODEX", c.Scope, path, func(w io.Writer) error {
			return codexconfig.Install(w, fs, logger, codexconfig.InstallOptions{
				Scope:  c.Scope,
				DryRun: c.DryRun,
			})
		})
	default:
		return fmt.Errorf("unknown agent %q", a)
	}
}

func (c *Cmd) writeAgent(stdout io.Writer, label, scope, path string, install func(io.Writer) error) error {
	if c.DryRun {
		var buf bytes.Buffer
		if err := install(&buf); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s %s %s\n", label, scope, path)
		_, err := stdout.Write(buf.Bytes())
		return err
	}
	if err := install(io.Discard); err != nil {
		return err
	}
	_, err := fmt.Fprintf(stdout, "%s %s %s installed\n", label, scope, path)
	return err
}
