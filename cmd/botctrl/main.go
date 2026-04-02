// The botctrl command is a CLI tool for managing AI coding assistants.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"

	"github.com/ajbeck/botctrl/internal/cmd/claude"
	"github.com/ajbeck/botctrl/internal/version"
)

type cli struct {
	Version versionFlag `name:"version" help:"Print version and exit." short:"v"`
	Claude  claude.Cmd  `cmd:"claude" help:"Claude Code agent commands — hooks, status line, and configuration."`
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
	ctx := kong.Parse(&c,
		kong.Name("botctrl"),
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
	ctx.FatalIfErrorf(ctx.Run())
}
