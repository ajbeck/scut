// Package format implements the "format" command group.
package format

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ajbeck/scut/internal/format"
	"github.com/ajbeck/scut/internal/formatignore"
	"github.com/spf13/afero"
)

// Cmd is the Kong command group for "scut format".
type Cmd struct {
	Go       goCmd       `cmd:"go" help:"Format Go source files. Reads from stdin if no files specified."`
	Markdown markdownCmd `cmd:"markdown" help:"Format Markdown files. Reads from stdin if no files specified."`
}

type goCmd struct {
	Files []string `arg:"" optional:"" name:"file" help:"Files to format. If omitted, reads from stdin."`
	Force bool     `help:"Format files even when ignored by .prettierignore or .scutignore."`
}

func (c *goCmd) Run(stdin io.Reader, stdout io.Writer) error {
	if len(c.Files) == 0 {
		return formatStdin(stdout, stdin, format.FormatGo)
	}
	return formatFiles(stdout, c.Files, format.FormatGo, c.Force)
}

type markdownCmd struct {
	Files []string `arg:"" optional:"" name:"file" help:"Files to format. If omitted, reads from stdin."`
	Force bool     `help:"Format files even when ignored by .prettierignore or .scutignore."`
}

func (c *markdownCmd) Run(stdin io.Reader, stdout io.Writer) error {
	if len(c.Files) == 0 {
		return formatStdin(stdout, stdin, format.FormatMarkdown)
	}
	return formatFiles(stdout, c.Files, format.FormatMarkdown, c.Force)
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

func formatFiles(w io.Writer, files []string, fn formatter, force bool) error {
	fs := afero.NewOsFs()
	for _, path := range files {
		if !force {
			ignored, err := ignoredPath(fs, path)
			if err != nil {
				return fmt.Errorf("checking ignores for %s: %w", path, err)
			}
			if ignored {
				continue
			}
		}

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

func ignoredPath(fs afero.Fs, path string) (bool, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	info, err := fs.Stat(abs)
	if err != nil {
		return false, err
	}
	return formatignore.MatchPath(fs, abs, info.IsDir())
}
