// The scut command is a CLI tool for managing AI coding assistants.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"

	"github.com/ajbeck/scut/internal/cmd/claude"
	"github.com/ajbeck/scut/internal/cmd/codex"
	doctorcmd "github.com/ajbeck/scut/internal/cmd/doctor"
	formatcmd "github.com/ajbeck/scut/internal/cmd/format"
	gotoolscmd "github.com/ajbeck/scut/internal/cmd/gotools"
	initcmd "github.com/ajbeck/scut/internal/cmd/initcmd"
	loggingcmd "github.com/ajbeck/scut/internal/cmd/logging"
	updatecmd "github.com/ajbeck/scut/internal/cmd/update"
	versioncmd "github.com/ajbeck/scut/internal/cmd/version"
	"github.com/ajbeck/scut/internal/logging"
	"github.com/ajbeck/scut/internal/version"
)

type cli struct {
	VersionFlag versionFlag    `name:"version" help:"Print version and exit." short:"v"`
	Version     versioncmd.Cmd `cmd:"version" help:"Print version and exit."`
	Claude      claude.Cmd     `cmd:"claude" help:"Claude Code agent commands — hooks, status line, and configuration."`
	Codex       codex.Cmd      `cmd:"codex" help:"Codex agent commands — hooks and lifecycle integrations."`
	Init        initcmd.Cmd    `cmd:"init" help:"Set up scut hooks for detected or selected coding agents."`
	Doctor      doctorcmd.Cmd  `cmd:"doctor" help:"Diagnose scut hook setup for supported coding agents."`
	Format      formatcmd.Cmd  `cmd:"format" help:"Format source code files."`
	Gotools     gotoolscmd.Cmd `cmd:"gotools" help:"Go tool-inspired commands for agents."`
	Logging     loggingcmd.Cmd `cmd:"logging" help:"Manage scut log files."`
	Update      updatecmd.Cmd  `cmd:"update" help:"Update scut when the install method supports automatic updates."`
}

// versionFlag is a kong flag type that prints the version and exits.
type versionFlag bool

func (v versionFlag) BeforeReset(app *kong.Kong, vars kong.Vars) error {
	fmt.Fprintln(app.Stdout, vars["version"])
	app.Exit(0)
	return nil
}

func main() {
	var c cli
	parser := kong.Must(&c,
		kong.Name("scut"),
		kong.Description("CLI tool for managing AI coding agents. Called as a subprocess by agent hooks — reads JSON from stdin, writes JSON to stdout."),
		kong.Vars{"version": version.String()},
		kong.BindTo(os.Stdin, (*io.Reader)(nil)),
		kong.BindTo(os.Stdout, (*io.Writer)(nil)),
		kong.BindTo(afero.NewOsFs(), (*afero.Fs)(nil)),
		kong.HelpOptions{
			NoExpandSubcommands: true,
			FlagsLast:           true,
			Compact:             true,
		},
	)

	ctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		logging.LogParseError(os.Args, err)
		parser.FatalIfErrorf(err)
	}

	logger, logCloser := c.openLogger(ctx.Command())
	if logCloser != nil {
		defer logCloser.Close()
	}

	logger.Debug("invoked", "args", os.Args, "command", ctx.Command())

	ctx.FatalIfErrorf(ctx.Run(logger))
}

func (c *cli) openLogger(command string) (*slog.Logger, io.Closer) {
	if command == "codex" || strings.HasPrefix(command, "codex ") {
		return c.Codex.OpenLogger(command)
	}
	return c.Claude.OpenLogger(command)
}
