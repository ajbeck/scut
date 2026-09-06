package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"

	versionmeta "github.com/ajbeck/scut/internal/version"
)

func TestVersionCommand(t *testing.T) {
	orig, origMeta := versionmeta.Version, versionmeta.BuildMetadata
	t.Cleanup(func() {
		versionmeta.Version = orig
		versionmeta.BuildMetadata = origMeta
	})

	versionmeta.Version = "v2.3.4"
	versionmeta.BuildMetadata = "def456"

	var c cli
	var stdout bytes.Buffer
	parser := kong.Must(&c,
		kong.Name("scut"),
		kong.Vars{"version": versionmeta.String()},
		kong.BindTo(&stdout, (*io.Writer)(nil)),
		kong.BindTo(afero.NewMemMapFs(), (*afero.Fs)(nil)),
	)

	ctx, err := parser.Parse([]string{"version"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := ctx.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got, want := stdout.String(), "v2.3.4+def456\n"; got != want {
		t.Errorf("version command wrote %q, want %q", got, want)
	}
}

func TestGotoolsDocCommandParses(t *testing.T) {
	var c cli
	var stdout bytes.Buffer
	parser := kong.Must(&c,
		kong.Name("scut"),
		kong.Vars{"version": versionmeta.String()},
		kong.BindTo(&stdout, (*io.Writer)(nil)),
		kong.BindTo(afero.NewMemMapFs(), (*afero.Fs)(nil)),
	)

	ctx, err := parser.Parse([]string{"gotools", "doc", "encoding/json"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got, want := ctx.Command(), "gotools doc <lookup>"; got != want {
		t.Errorf("Command() = %q, want %q", got, want)
	}
}

func TestGotoolsCacheCommandsParse(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"gotools", "cache", "path"}, want: "gotools cache path"},
		{args: []string{"gotools", "cache", "list", "example.com/acme/tool", "--json"}, want: "gotools cache list <module>"},
		{args: []string{"gotools", "cache", "verify"}, want: "gotools cache verify"},
		{args: []string{"gotools", "cache", "remove", "example.com/acme/tool@v1.2.3"}, want: "gotools cache remove <module>"},
		{args: []string{"gotools", "cache", "clean"}, want: "gotools cache clean"},
		{args: []string{"gotools", "cache", "prune", "--older-than=720h"}, want: "gotools cache prune"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			var c cli
			parser := kong.Must(&c,
				kong.Name("scut"),
				kong.Vars{"version": versionmeta.String()},
				kong.BindTo(&bytes.Buffer{}, (*io.Writer)(nil)),
				kong.BindTo(afero.NewMemMapFs(), (*afero.Fs)(nil)),
			)
			ctx, err := parser.Parse(tt.args)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.args, err)
			}
			if got := ctx.Command(); got != tt.want {
				t.Fatalf("Command() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRootHelpSeparatesCommandsAndCommandGroups(t *testing.T) {
	var c cli
	var stdout bytes.Buffer
	parser := kong.Must(&c,
		kong.Name("scut"),
		kong.Description("CLI tool for managing AI coding agents, hooks, MCP utilities, and developer helper commands."),
		kong.Vars{"version": versionmeta.String()},
		kong.BindTo(&stdout, (*io.Writer)(nil)),
		kong.BindTo(afero.NewMemMapFs(), (*afero.Fs)(nil)),
		kong.Writers(&stdout, &stdout),
		kong.Help(rootHelpPrinter),
		kong.HelpOptions{
			NoExpandSubcommands: true,
			FlagsLast:           true,
			Compact:             true,
		},
		kong.Exit(func(int) {
			panic("exit")
		}),
	)

	func() {
		defer func() {
			if recovered := recover(); recovered != "exit" {
				t.Fatalf("Parse() panic = %v, want exit", recovered)
			}
		}()
		_, err := parser.Parse([]string{"--help"})
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
	}()

	help := stdout.String()
	if !strings.Contains(help, "Usage: scut <command-or-group> [flags]") {
		t.Fatalf("root help = %q, want command-or-group usage", help)
	}
	commands := strings.Index(help, "Commands:")
	commandGroups := strings.Index(help, "Command groups:")
	if commands < 0 || commandGroups < 0 {
		t.Fatalf("root help = %q, want Commands and Command groups sections", help)
	}
	if commands > commandGroups {
		t.Fatalf("Commands section appears after Command groups section:\n%s", help)
	}
	if !strings.Contains(help, "  version    Print version and exit.") ||
		!strings.Contains(help, "  update     Update scut when the install method supports automatic updates.") {
		t.Fatalf("root help Commands section missing top-level commands:\n%s", help)
	}
	if !strings.Contains(help, "  claude     Claude Code agent commands") ||
		!strings.Contains(help, "  mcp        MCP utility commands for agents.") {
		t.Fatalf("root help Command groups section missing command groups:\n%s", help)
	}
}

func TestMCPAWSProxyCommandParses(t *testing.T) {
	var c cli
	var stdout bytes.Buffer
	parser := kong.Must(&c,
		kong.Name("scut"),
		kong.Vars{"version": versionmeta.String()},
		kong.BindTo(&stdout, (*io.Writer)(nil)),
		kong.BindTo(afero.NewMemMapFs(), (*afero.Fs)(nil)),
	)

	ctx, err := parser.Parse([]string{
		"mcp",
		"aws-proxy",
		"https://aws-mcp.us-east-1.api.aws/mcp",
		"--skip-auth",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got, want := ctx.Command(), "mcp aws-proxy <endpoint>"; got != want {
		t.Errorf("Command() = %q, want %q", got, want)
	}
}

func TestUpdateCommandParses(t *testing.T) {
	var c cli
	var stdout bytes.Buffer
	parser := kong.Must(&c,
		kong.Name("scut"),
		kong.Vars{"version": versionmeta.String()},
		kong.BindTo(&stdout, (*io.Writer)(nil)),
		kong.BindTo(afero.NewMemMapFs(), (*afero.Fs)(nil)),
	)

	ctx, err := parser.Parse([]string{"update", "--dry-run", "--target-version", "v1.2.3"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got, want := ctx.Command(), "update"; got != want {
		t.Errorf("Command() = %q, want %q", got, want)
	}
}

func TestCodexHookCommandParses(t *testing.T) {
	var c cli
	var stdout bytes.Buffer
	parser := kong.Must(&c,
		kong.Name("scut"),
		kong.Vars{"version": versionmeta.String()},
		kong.BindTo(strings.NewReader(`{"session_id":"test","hook_event_name":"Stop","turn_id":"turn-1"}`), (*io.Reader)(nil)),
		kong.BindTo(&stdout, (*io.Writer)(nil)),
		kong.BindTo(afero.NewMemMapFs(), (*afero.Fs)(nil)),
	)

	ctx, err := parser.Parse([]string{"codex", "hook", "stop"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got, want := ctx.Command(), "codex hook stop"; got != want {
		t.Errorf("Command() = %q, want %q", got, want)
	}
}
