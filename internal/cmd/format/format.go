// Package format implements the "format" command group.
package format

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
	Files []string `arg:"" optional:"" name:"file" help:"Files to format in place. If omitted, reads from stdin and writes to stdout."`
	Force bool     `help:"Format files even when ignored by .prettierignore or .scutignore."`
	Check bool     `help:"Check whether named files are formatted without writing changes."`
}

func (c *goCmd) Run(stdin io.Reader, stdout io.Writer, fs afero.Fs) error {
	if len(c.Files) == 0 {
		if c.Check {
			return fmt.Errorf("--check requires at least one file")
		}
		return formatStdin(stdout, stdin, format.FormatGo)
	}
	return formatFiles(fs, c.Files, format.FormatGo, c.Force, c.Check)
}

type markdownCmd struct {
	Files       []string `arg:"" optional:"" name:"file" help:"Files to format in place. If omitted, reads from stdin and writes to stdout."`
	Force       bool     `help:"Format files even when ignored by .prettierignore or .scutignore."`
	Check       bool     `help:"Check whether named files are formatted without writing changes."`
	ProseWrap   string   `name:"prose-wrap" default:"preserve" enum:"preserve,always,never" help:"How to wrap prose: preserve, always, or never."`
	PrintWidth  *int     `name:"print-width" default:"80" help:"Target line width for prose wrapping and compact tables."`
	TabWidth    *int     `name:"tab-width" default:"2" help:"Tab width used for list indentation alignment."`
	SingleQuote bool     `name:"single-quote" help:"Use single quotes for link and image titles."`
}

func (c *markdownCmd) Run(stdin io.Reader, stdout io.Writer, fs afero.Fs) error {
	config, err := c.formatConfig()
	if err != nil {
		return err
	}
	if len(c.Files) == 0 {
		if c.Check {
			return fmt.Errorf("--check requires at least one file")
		}
		return formatStdin(stdout, stdin, markdownFormatter(config))
	}
	return formatFiles(fs, c.Files, markdownFormatter(config), c.Force, c.Check)
}

func (c *markdownCmd) Validate() error {
	if c.Check && len(c.Files) == 0 {
		return fmt.Errorf("--check requires at least one file")
	}
	_, err := c.formatConfig()
	return err
}

func (c *markdownCmd) formatConfig() (format.MarkdownConfig, error) {
	config := format.DefaultMarkdownConfig()
	if c.ProseWrap != "" {
		config.ProseWrap = format.MarkdownProseWrap(c.ProseWrap)
	}
	if c.PrintWidth != nil {
		config.PrintWidth = *c.PrintWidth
	}
	if c.TabWidth != nil {
		config.TabWidth = *c.TabWidth
	}
	if config.PrintWidth <= 0 {
		return format.MarkdownConfig{}, fmt.Errorf("--print-width must be greater than zero")
	}
	if config.TabWidth <= 0 {
		return format.MarkdownConfig{}, fmt.Errorf("--tab-width must be greater than zero")
	}
	config.SingleQuote = c.SingleQuote
	if err := config.Validate(); err != nil {
		return format.MarkdownConfig{}, err
	}
	return config, nil
}

func markdownFormatter(config format.MarkdownConfig) formatter {
	return func(src []byte) ([]byte, error) {
		return format.FormatMarkdownWithConfig(src, config)
	}
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
		formatted = src
	}
	_, err = w.Write(formatted)
	return err
}

func formatFiles(fs afero.Fs, files []string, fn formatter, force, check bool) error {
	var changed []string
	changedSet := map[string]struct{}{}
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

		target, err := resolveFilePath(fs, path)
		if err != nil {
			return fmt.Errorf("resolving %s: %w", path, err)
		}
		info, err := fs.Stat(target)
		if err != nil {
			return fmt.Errorf("stating %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("formatting %s: not a regular file", path)
		}

		src, err := afero.ReadFile(fs, target)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		formatted, err := fn(src)
		if err != nil {
			return fmt.Errorf("formatting %s: %w", path, err)
		}
		if formatted == nil || bytes.Equal(src, formatted) {
			continue
		}
		if check {
			if _, reported := changedSet[path]; !reported {
				changed = append(changed, path)
				changedSet[path] = struct{}{}
			}
			continue
		}
		if err := writeFileAtomic(fs, target, formatted, info.Mode()); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	if len(changed) > 0 {
		return &checkError{files: changed}
	}
	return nil
}

type checkError struct {
	files []string
}

func (e *checkError) Error() string {
	return "format check failed; files would change: " + strings.Join(e.files, ", ")
}

func resolveFilePath(fs afero.Fs, path string) (string, error) {
	const maxSymlinks = 255
	for range maxSymlinks {
		lstater, ok := fs.(afero.Lstater)
		if !ok {
			return path, nil
		}
		info, usedLstat, err := lstater.LstatIfPossible(path)
		if err != nil {
			return "", err
		}
		if !usedLstat || info.Mode()&os.ModeSymlink == 0 {
			return path, nil
		}

		linkReader, ok := fs.(afero.LinkReader)
		if !ok {
			return "", fmt.Errorf("filesystem cannot resolve symbolic link")
		}
		target, err := linkReader.ReadlinkIfPossible(path)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		path = filepath.Clean(target)
	}
	return "", fmt.Errorf("too many symbolic links")
}

func writeFileAtomic(fs afero.Fs, path string, content []byte, mode os.FileMode) (err error) {
	temp, err := afero.TempFile(fs, filepath.Dir(path), "."+filepath.Base(path)+".scut-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	closed := false
	renamed := false
	defer func() {
		if !closed {
			_ = temp.Close()
		}
		if !renamed {
			_ = fs.Remove(tempPath)
		}
	}()

	if _, err := io.Copy(temp, bytes.NewReader(content)); err != nil {
		return err
	}
	if err := fs.Chmod(tempPath, mode); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		closed = true
		return err
	}
	closed = true
	if err := fs.Rename(tempPath, path); err != nil {
		return err
	}
	renamed = true
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
