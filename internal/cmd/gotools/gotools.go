// Package gotools implements Go tool-inspired commands.
package gotools

import (
	"context"
	"io"
	"strings"

	"github.com/spf13/afero"

	"github.com/ajbeck/scut/internal/godoc"
)

// Cmd is the Kong command group for "scut gotools".
type Cmd struct {
	Doc docCmd `cmd:"doc" help:"Show Go documentation for a package or symbol."`
}

type docCmd struct {
	Args    []string `arg:"" optional:"" name:"lookup" help:"Optional package, symbol, or package symbol lookup."`
	All     bool     `help:"Show all documentation for the package."`
	Short   bool     `help:"Show one-line representation for each symbol."`
	Src     bool     `help:"Show full source for the selected symbol."`
	U       bool     `short:"u" help:"Show unexported symbols as well as exported symbols."`
	C       bool     `short:"c" help:"Respect case when matching symbols."`
	Cmd     bool     `help:"Show symbols with package docs even if package is a command."`
	Version string   `name:"module-version" help:"Module version query for external packages." default:"latest"`
}

type docClient interface {
	Doc(context.Context, godoc.Options) (string, error)
}

var newDocClient = func(fs afero.Fs) (docClient, error) {
	return godoc.NewDefaultClient(fs)
}

func (c *docCmd) Run(stdout io.Writer, fs afero.Fs) error {
	client, err := newDocClient(fs)
	if err != nil {
		return err
	}

	out, err := client.Doc(context.Background(), godoc.Options{
		Args:          c.Args,
		Version:       c.Version,
		All:           c.All,
		Short:         c.Short,
		Src:           c.Src,
		Unexported:    c.U,
		CaseSensitive: c.C,
		Cmd:           c.Cmd,
	})
	if err != nil {
		return err
	}
	_, err = io.WriteString(stdout, strings.TrimRight(out, "\n")+"\n")
	return err
}
