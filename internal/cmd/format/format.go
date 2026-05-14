// Package format implements the "format" command group.
package format

import (
	"fmt"
	"io"
	"os"

	"github.com/ajbeck/botctrl/internal/format"
)

// Cmd is the Kong command group for "botctrl format".
type Cmd struct {
	Go       goCmd       `cmd:"go" help:"Format Go source files. Reads from stdin if no files specified."`
	Markdown markdownCmd `cmd:"markdown" help:"Format Markdown files. Reads from stdin if no files specified."`
}

type goCmd struct {
	Files []string `arg:"" optional:"" name:"file" help:"Files to format. If omitted, reads from stdin."`
}

func (c *goCmd) Run(stdin io.Reader, stdout io.Writer) error {
	if len(c.Files) == 0 {
		return formatStdin(stdout, stdin, format.FormatGo)
	}
	return formatFiles(stdout, c.Files, format.FormatGo)
}

type markdownCmd struct {
	Files []string `arg:"" optional:"" name:"file" help:"Files to format. If omitted, reads from stdin."`
}

func (c *markdownCmd) Run(stdin io.Reader, stdout io.Writer) error {
	if len(c.Files) == 0 {
		return formatStdin(stdout, stdin, format.FormatMarkdown)
	}
	return formatFiles(stdout, c.Files, format.FormatMarkdown)
}

type formatter func([]byte) ([]byte, error)

func formatStdin(w io.Writer, r io.Reader, fn formatter) error {
	src, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}
	formatted, err := fn(src)
	if err != nil {
		return err
	}
	if formatted == nil {
		return nil
	}
	_, err = w.Write(formatted)
	return err
}

func formatFiles(w io.Writer, files []string, fn formatter) error {
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		formatted, err := fn(src)
		if err != nil {
			return err
		}
		if formatted == nil {
			continue
		}
		_, err = w.Write(formatted)
		if err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}