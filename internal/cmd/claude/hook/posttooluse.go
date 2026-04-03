package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"time"

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

func (c *postToolUseCmd) Run(stdin io.Reader, stdout io.Writer, fs afero.Fs, logger *slog.Logger) error {
	start := time.Now()

	var in cc.PostToolUseInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PostToolUse input: %w", err)
	}

	logger = logger.With("hook", "post-tool-use", "session_id", in.SessionID, "tool_name", in.ToolName)

	fp := in.FilePath()
	if fp == "" {
		logger.Debug("skipped", "reason", "no file_path", "duration_ms", ms(start))
		return writeJSON(stdout, cc.PostToolUseOutput{})
	}

	info, err := fs.Stat(fp)
	if err != nil {
		logger.Debug("skipped", "reason", "file not found", "file_path", fp, "duration_ms", ms(start))
		return writeJSON(stdout, cc.PostToolUseOutput{})
	}

	var formatter func([]byte) ([]byte, error)
	var formatterName string
	switch filepath.Ext(fp) {
	case ".go":
		formatter = formatGo
		formatterName = "gofmt"
	case ".md", ".mdx":
		formatter = formatMarkdown
		formatterName = "markdown"
	default:
		logger.Debug("skipped", "reason", "unsupported extension", "file_path", fp, "duration_ms", ms(start))
		return writeJSON(stdout, cc.PostToolUseOutput{})
	}

	src, err := afero.ReadFile(fs, fp)
	if err != nil {
		logger.Warn("read failed", "file_path", fp, "error", err, "duration_ms", ms(start))
		return writeJSON(stdout, cc.PostToolUseOutput{})
	}

	formatted, err := formatter(src)
	if err != nil || formatted == nil || bytes.Equal(src, formatted) {
		logger.Debug("unchanged", "file_path", fp, "formatter", formatterName, "duration_ms", ms(start))
		return writeJSON(stdout, cc.PostToolUseOutput{})
	}

	_ = afero.WriteFile(fs, fp, formatted, info.Mode())
	logger.Info("formatted", "file_path", fp, "formatter", formatterName, "duration_ms", ms(start))
	return writeJSON(stdout, cc.PostToolUseOutput{})
}
