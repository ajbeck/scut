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
	Claude       bool   `help:"Set up Claude Code hooks."`
	Codex        bool   `help:"Set up Codex hooks."`
	All          bool   `help:"Set up all supported agents, even if not detected."`
	Scope        string `help:"Configuration scope: project or user." default:"project" enum:"project,user"`
	BakeLog      bool   `help:"Bake --log into generated command strings." name:"bake-log"`
	BakeLogLevel string `help:"Bake --log-level=LEVEL into generated command strings (implies --bake-log). One of: debug, info, warn, error." placeholder:"LEVEL" name:"bake-log-level" enum:",debug,info,warn,error" default:""`
	DryRun       bool   `help:"Print the planned config output without writing files." name:"dry-run"`
	Verbose      bool   `help:"With --dry-run, print the full rendered config for each selected agent."`
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
	plans, err := c.planAgents(agents)
	if err != nil {
		return err
	}
	if !c.DryRun {
		if err := preflightPlans(fs, plans); err != nil {
			return err
		}
	}
	for _, plan := range plans {
		if err := c.installAgent(stdout, fs, logger, plan); err != nil {
			return err
		}
	}
	if !c.DryRun {
		fmt.Fprintf(stdout, "NEXT   run scut doctor --scope=%s\n", c.Scope)
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
		return exists(fs, configDir)
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

type installPlan struct {
	Agent agent
	Label string
	Scope string
	Path  string
}

func (c *Cmd) planAgents(agents []agent) ([]installPlan, error) {
	plans := make([]installPlan, 0, len(agents))
	for _, a := range agents {
		switch a {
		case agentClaude:
			path, err := claudeconfig.PathForScope(c.Scope)
			if err != nil {
				return nil, err
			}
			plans = append(plans, installPlan{Agent: a, Label: "CLAUDE", Scope: c.Scope, Path: path})
		case agentCodex:
			path, err := codexconfig.PathForScope(c.Scope)
			if err != nil {
				return nil, err
			}
			plans = append(plans, installPlan{Agent: a, Label: "CODEX", Scope: c.Scope, Path: path})
		default:
			return nil, fmt.Errorf("unknown agent %q", a)
		}
	}
	return plans, nil
}

func preflightPlans(fs afero.Fs, plans []installPlan) error {
	for _, plan := range plans {
		if err := preflightWritable(fs, plan.Path); err != nil {
			return fmt.Errorf("preflight %s %s %q: %w", plan.Label, plan.Scope, plan.Path, err)
		}
	}
	return nil
}

func preflightWritable(fs afero.Fs, path string) error {
	if info, err := fs.Stat(path); err == nil {
		if info.IsDir() {
			return fmt.Errorf("target is a directory")
		}
		file, err := fs.OpenFile(path, os.O_RDWR, 0644)
		if err != nil {
			return err
		}
		return file.Close()
	}
	dir := filepath.Dir(path)
	if err := fs.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := afero.TempFile(fs, dir, ".scut-preflight-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	closeErr := tmp.Close()
	removeErr := fs.Remove(name)
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}

func (c *Cmd) installAgent(stdout io.Writer, fs afero.Fs, logger *slog.Logger, plan installPlan) error {
	a := plan.Agent
	switch a {
	case agentClaude:
		return c.writeAgent(stdout, plan, func(w io.Writer) error {
			return claudeconfig.Install(w, fs, logger, claudeconfig.InstallOptions{
				Scope:        c.Scope,
				BakeLog:      c.BakeLog,
				BakeLogLevel: c.BakeLogLevel,
				DryRun:       c.DryRun,
			})
		})
	case agentCodex:
		return c.writeAgent(stdout, plan, func(w io.Writer) error {
			return codexconfig.Install(w, fs, logger, codexconfig.InstallOptions{
				Scope:        c.Scope,
				BakeLog:      c.BakeLog,
				BakeLogLevel: c.BakeLogLevel,
				DryRun:       c.DryRun,
			})
		})
	default:
		return fmt.Errorf("unknown agent %q", a)
	}
}

func (c *Cmd) writeAgent(stdout io.Writer, plan installPlan, install func(io.Writer) error) error {
	if c.DryRun {
		var buf bytes.Buffer
		if err := install(&buf); err != nil {
			return err
		}
		if !c.Verbose {
			fmt.Fprintf(stdout, "%-6s %-7s %s would update\n", plan.Label, plan.Scope, plan.Path)
			return nil
		}
		fmt.Fprintf(stdout, "%s %s %s\n", plan.Label, plan.Scope, plan.Path)
		_, err := stdout.Write(buf.Bytes())
		return err
	}
	if err := install(io.Discard); err != nil {
		return err
	}
	_, err := fmt.Fprintf(stdout, "%-6s %-7s %s installed\n", plan.Label, plan.Scope, plan.Path)
	return err
}
