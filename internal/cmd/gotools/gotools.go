// Package gotools implements Go tool-inspired commands.
package gotools

import (
	"errors"
	"fmt"
	"io"
	"strings"
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

func (c *docCmd) Run(stdout io.Writer) error {
	if c.Package == "" {
		return errors.New("package is required")
	}

	args := []string{c.Package}
	if c.Symbol != "" {
		args = append(args, c.Symbol)
	}

	_, err := fmt.Fprintf(stdout, "gotools doc placeholder: %s\n", strings.Join(args, " "))
	return err
}
