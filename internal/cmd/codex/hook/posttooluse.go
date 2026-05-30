//go:build goexperiment.jsonv2

package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"

	cx "github.com/ajbeck/scut/hooks/codex"
	"github.com/ajbeck/scut/internal/format"
	"github.com/ajbeck/scut/internal/formatignore"
)

type postToolUseCmd struct{ trailingArgs }

func (c *postToolUseCmd) Help() string {
	return `Formats files in place after successful Codex file edits.
Dispatches by file extension: .go files are formatted with gofmt,
.md and .mdx files are formatted with goldmark-prettier-markdown.
Paths matched by root .prettierignore or .scutignore files are skipped.
Files with other extensions or syntax errors are left unchanged.`
}

func (c *postToolUseCmd) Run(stdin io.Reader, stdout io.Writer, fs afero.Fs, logger *slog.Logger) error {
	start := time.Now()
	var in cx.PostToolUseInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decoding PostToolUse input: %w", err)
	}

	logger = logger.With("hook", "post-tool-use", "session_id", in.SessionID, "turn_id", in.TurnID, "tool_name", in.ToolName)
	paths := in.FilePaths()
	if len(paths) == 0 {
		logger.Debug("skipped", "reason", "no file paths", "duration_ms", ms(start))
		return writeJSON(stdout, cx.PostToolUseOutput{})
	}

	var formatted []string
	for _, fp := range paths {
		changed, formatterName := formatPath(fs, fp, logger)
		if changed {
			formatted = append(formatted, fp)
			logger.Info("formatted", "file_path", fp, "formatter", formatterName)
		}
	}
	if len(formatted) == 0 {
		logger.Debug("unchanged", "file_count", len(paths), "duration_ms", ms(start))
		return writeJSON(stdout, cx.PostToolUseOutput{})
	}

	return writeJSON(stdout, cx.PostToolUseOutput{
		HookSpecificOutput: &cx.ContextHookOutput{
			HookEventName:     cx.EventPostToolUse,
			AdditionalContext: new(fmt.Sprintf("scut formatted %s after the %s tool completed. The files on disk may differ from the original tool input.", strings.Join(formatted, ", "), in.ToolName)),
		},
	})
}

func formatPath(fs afero.Fs, fp string, logger *slog.Logger) (bool, string) {
	info, err := fs.Stat(fp)
	if err != nil {
		logger.Debug("skipped", "reason", "file not found", "file_path", fp)
		return false, ""
	}

	ignored, err := formatignore.MatchPath(fs, fp, info.IsDir())
	if err != nil {
		logger.Warn("ignore check failed", "file_path", fp, "error", err)
	} else if ignored {
		logger.Debug("skipped", "reason", "ignored", "file_path", fp)
		return false, ""
	}

	var formatter func([]byte) ([]byte, error)
	var formatterName string
	switch filepath.Ext(fp) {
	case ".go":
		formatter = format.FormatGo
		formatterName = "gofmt"
	case ".md", ".mdx":
		formatter = format.FormatMarkdown
		formatterName = "markdown"
	default:
		logger.Debug("skipped", "reason", "unsupported extension", "file_path", fp)
		return false, ""
	}

	src, err := afero.ReadFile(fs, fp)
	if err != nil {
		logger.Warn("read failed", "file_path", fp, "error", err)
		return false, ""
	}

	formatted, err := formatter(src)
	if err != nil || formatted == nil || bytes.Equal(src, formatted) {
		logger.Debug("unchanged", "file_path", fp, "formatter", formatterName)
		return false, ""
	}

	if err := afero.WriteFile(fs, fp, formatted, info.Mode()); err != nil {
		logger.Warn("write failed", "file_path", fp, "formatter", formatterName, "error", err)
		return false, ""
	}
	return true, formatterName
}
