//go:build mage

// Documentation targets.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/sh"
)

type cliHelpDoc struct {
	Name    string
	Command string
	Args    []string
}

var cliHelpDocs = []cliHelpDoc{
	{Name: "scut", Command: "scut --help", Args: []string{"--help"}},
	{Name: "scut-version", Command: "scut version --help", Args: []string{"version", "--help"}},
	{Name: "scut-claude", Command: "scut claude --help", Args: []string{"claude", "--help"}},
	{Name: "scut-claude-status-line", Command: "scut claude status-line --help", Args: []string{"claude", "status-line", "--help"}},
	{Name: "scut-claude-config", Command: "scut claude config --help", Args: []string{"claude", "config", "--help"}},
	{Name: "scut-claude-config-install", Command: "scut claude config install --help", Args: []string{"claude", "config", "install", "--help"}},
	{Name: "scut-claude-config-uninstall", Command: "scut claude config uninstall --help", Args: []string{"claude", "config", "uninstall", "--help"}},
	{Name: "scut-claude-config-status", Command: "scut claude config status --help", Args: []string{"claude", "config", "status", "--help"}},
	{Name: "scut-claude-hook", Command: "scut claude hook --help", Args: []string{"claude", "hook", "--help"}},
	{Name: "scut-codex", Command: "scut codex --help", Args: []string{"codex", "--help"}},
	{Name: "scut-codex-config", Command: "scut codex config --help", Args: []string{"codex", "config", "--help"}},
	{Name: "scut-codex-config-install", Command: "scut codex config install --help", Args: []string{"codex", "config", "install", "--help"}},
	{Name: "scut-codex-config-uninstall", Command: "scut codex config uninstall --help", Args: []string{"codex", "config", "uninstall", "--help"}},
	{Name: "scut-codex-config-status", Command: "scut codex config status --help", Args: []string{"codex", "config", "status", "--help"}},
	{Name: "scut-codex-hook", Command: "scut codex hook --help", Args: []string{"codex", "hook", "--help"}},
	{Name: "scut-init", Command: "scut init --help", Args: []string{"init", "--help"}},
	{Name: "scut-doctor", Command: "scut doctor --help", Args: []string{"doctor", "--help"}},
	{Name: "scut-update", Command: "scut update --help", Args: []string{"update", "--help"}},
	{Name: "scut-format", Command: "scut format --help", Args: []string{"format", "--help"}},
	{Name: "scut-format-go", Command: "scut format go --help", Args: []string{"format", "go", "--help"}},
	{Name: "scut-format-markdown", Command: "scut format markdown --help", Args: []string{"format", "markdown", "--help"}},
	{Name: "scut-gotools", Command: "scut gotools --help", Args: []string{"gotools", "--help"}},
	{Name: "scut-gotools-doc", Command: "scut gotools doc --help", Args: []string{"gotools", "doc", "--help"}},
	{Name: "scut-logging", Command: "scut logging --help", Args: []string{"logging", "--help"}},
	{Name: "scut-logging-clean", Command: "scut logging clean --help", Args: []string{"logging", "clean", "--help"}},
}

// Docs builds the Hugo documentation site into public/.
func Docs(ctx context.Context) error {
	if err := DocsCLIHelp(ctx); err != nil {
		return err
	}
	return sh.Run("hugo", "--source", "docs", "--gc", "--minify")
}

// DocsCLIHelp regenerates Hugo assets from the scut --help output.
func DocsCLIHelp(ctx context.Context) error {
	if err := Build(ctx); err != nil {
		return err
	}

	dir := filepath.Join("docs", "assets", "cli-help")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	binary := filepath.Join(".", buildDir, binaryName)
	for _, doc := range cliHelpDocs {
		out, err := sh.Output(binary, doc.Args...)
		if err != nil {
			return fmt.Errorf("generate CLI help for %s: %w", doc.Command, err)
		}
		out = strings.TrimRight(out, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(dir, doc.Name+".txt"), []byte(out), 0o644); err != nil {
			return err
		}
	}
	return nil
}
