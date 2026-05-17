// Package gotools implements Go tool-inspired commands.
package gotools

import (
	"context"
	"errors"
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
	Package string `arg:"" optional:"" name:"package" help:"Package import path or local package path."`
	Symbol  string `arg:"" optional:"" name:"symbol" help:"Optional package symbol to show."`
	All     bool   `help:"Show all documentation for the package."`
	Short   bool   `help:"Show one-line representation for each symbol."`
	Src     bool   `help:"Show full source for the selected symbol."`
	U       bool   `short:"u" help:"Show unexported symbols as well as exported symbols."`
	C       bool   `short:"c" help:"Respect case when matching symbols."`
	Version string `name:"module-version" help:"Module version query for external packages." default:"latest"`
}

type docClient interface {
	Doc(context.Context, godoc.Options) (string, error)
}

var newDocClient = func(fs afero.Fs) (docClient, error) {
	return godoc.NewDefaultClient(fs)
}

func (c *docCmd) Run(stdout io.Writer, fs afero.Fs) error {
	if c.Package == "" {
		return errors.New("package is required")
	}

	client, err := newDocClient(fs)
	if err != nil {
		return err
	}

	out, err := client.Doc(context.Background(), godoc.Options{
		Package:       c.Package,
		Symbol:        c.Symbol,
		Version:       c.Version,
		All:           c.All,
		Short:         c.Short,
		Src:           c.Src,
		Unexported:    c.U,
		CaseSensitive: c.C,
	})
	if err != nil {
		return err
	}
	_, err = io.WriteString(stdout, strings.TrimRight(out, "\n")+"\n")
	return err
}
