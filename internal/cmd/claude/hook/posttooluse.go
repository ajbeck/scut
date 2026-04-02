package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/afero"

	cc "github.com/ajbeck/botctrl/hooks/claudecode"
)

type postToolUseCmd struct{}

func (c *postToolUseCmd) Help() string {
	return `Formats files in place after successful Write or Edit tool calls.
Dispatches by file extension: .go files are formatted with gofmt,
.md and .mdx files are formatted with goldmark-prettier-markdown.
Files with other extensions or syntax errors are left unchanged.`
}

func (c *postToolUseCmd) Run(stdin io.Reader, stdout io.Writer, fs afero.Fs) error {
	var in cc.PostToolUseInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PostToolUse input: %w", err)
	}

	fp := in.FilePath()
	if fp == "" {
		return writeJSON(stdout, cc.PostToolUseOutput{})
	}

	info, err := fs.Stat(fp)
	if err != nil {
		return writeJSON(stdout, cc.PostToolUseOutput{})
	}

	var formatter func([]byte) ([]byte, error)
	switch filepath.Ext(fp) {
	case ".go":
		formatter = formatGo
	case ".md", ".mdx":
		formatter = formatMarkdown
	default:
		return writeJSON(stdout, cc.PostToolUseOutput{})
	}

	src, err := afero.ReadFile(fs, fp)
	if err != nil {
		return writeJSON(stdout, cc.PostToolUseOutput{})
	}

	formatted, err := formatter(src)
	if err != nil || formatted == nil || bytes.Equal(src, formatted) {
		return writeJSON(stdout, cc.PostToolUseOutput{})
	}

	_ = afero.WriteFile(fs, fp, formatted, info.Mode())
	return writeJSON(stdout, cc.PostToolUseOutput{})
}
