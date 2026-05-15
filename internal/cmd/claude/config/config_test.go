//go:build goexperiment.jsonv2

// Package config_test exercises the public surface of the config command group
// via Kong wiring.
package config_test

import (
	"testing"

	"github.com/alecthomas/kong"

	"github.com/ajbeck/botctrl/internal/cmd/claude/config"
)

// kongSmokeCli is a minimal Kong root struct wrapping config.Cmd for parsing.
type kongSmokeCli struct {
	Config config.Cmd `cmd:""`
}

func TestKongWiring_InstallFlags(t *testing.T) {
	var cli kongSmokeCli
	parser := kong.Must(&cli,
		kong.Name("botctrl"),
	)

	ctx, err := parser.Parse([]string{
		"config", "install",
		"--dry-run",
		"--only=status-line",
		"--scope=project",
	})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Verify the parsed command path contains "config install".
	cmd := ctx.Command()
	want := "config install"
	if cmd != want {
		t.Errorf("ctx.Command() = %q, want %q", cmd, want)
	}
}

func TestKongWiring_ConfigCmdRegistered(t *testing.T) {
	// Verify that kong.Must does not panic — the command tree is valid.
	// This catches duplicate flags, missing enum defaults, etc.
	var cli kongSmokeCli
	_ = kong.Must(&cli, kong.Name("botctrl"))
}

func TestKongWiring_UninstallParseable(t *testing.T) {
	var cli kongSmokeCli
	parser := kong.Must(&cli, kong.Name("botctrl"))

	ctx, err := parser.Parse([]string{"config", "uninstall", "--scope=user"})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if ctx.Command() != "config uninstall" {
		t.Errorf("ctx.Command() = %q, want %q", ctx.Command(), "config uninstall")
	}
}

func TestKongWiring_StatusParseable(t *testing.T) {
	var cli kongSmokeCli
	parser := kong.Must(&cli, kong.Name("botctrl"))

	ctx, err := parser.Parse([]string{"config", "status", "--scope=both", "--json"})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if ctx.Command() != "config status" {
		t.Errorf("ctx.Command() = %q, want %q", ctx.Command(), "config status")
	}
}
