//go:build goexperiment.jsonv2

package hook

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/spf13/afero"

	cx "github.com/ajbeck/scut/hooks/codex"
	"github.com/ajbeck/scut/internal/logging"
)

type runHandler func(io.Reader, io.Writer) error

func stubHandler(fn func(io.Reader, io.Writer, *slog.Logger) error) runHandler {
	return func(r io.Reader, w io.Writer) error {
		return fn(r, w, logging.Discard)
	}
}

func fsHandler(fn func(io.Reader, io.Writer, afero.Fs, *slog.Logger) error) runHandler {
	return func(r io.Reader, w io.Writer) error {
		return fn(r, w, afero.NewMemMapFs(), logging.Discard)
	}
}

var minimalInput = `{"session_id":"test","hook_event_name":"test","turn_id":"turn-1"}`

func TestHandlers_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		run  runHandler
	}{
		{"session-start", stubHandler(new(sessionStartCmd).Run)},
		{"subagent-start", stubHandler(new(subagentStartCmd).Run)},
		{"pre-tool-use", stubHandler(new(preToolUseCmd).Run)},
		{"permission-request", stubHandler(new(permissionRequestCmd).Run)},
		{"post-tool-use", fsHandler(new(postToolUseCmd).Run)},
		{"pre-compact", stubHandler(new(preCompactCmd).Run)},
		{"post-compact", stubHandler(new(postCompactCmd).Run)},
		{"user-prompt-submit", stubHandler(new(userPromptSubmitCmd).Run)},
		{"subagent-stop", stubHandler(new(subagentStopCmd).Run)},
		{"stop", stubHandler(new(stopCmd).Run)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			if err := tt.run(strings.NewReader(minimalInput), &stdout); err != nil {
				t.Fatalf("Run() error: %v", err)
			}
			if !json.Valid(stdout.Bytes()) {
				t.Fatalf("output is not valid JSON: %s", stdout.String())
			}
		})
	}
}

func codexPostToolUsePayload(cwd string, toolInput json.RawMessage) string {
	in := cx.PostToolUseInput{
		TurnInput: cx.TurnInput{
			Input: cx.Input{
				SessionID:     "test-session",
				CWD:           cwd,
				HookEventName: cx.EventPostToolUse,
			},
			TurnID: "turn-1",
		},
		ToolName:  "apply_patch",
		ToolInput: toolInput,
	}
	data, _ := json.Marshal(in)
	return string(data)
}

func TestPostToolUseCmd_Dispatch(t *testing.T) {
	unformattedGo := "package main\n\nfunc main()  {}\n"
	formattedGo := "package main\n\nfunc main() {}\n"

	unformattedMd := "#  Hello\n\nworld\n"
	formattedMd := "# Hello\n\nworld\n"

	tests := []struct {
		name         string
		payload      string
		files        map[string]string
		extraFiles   map[string]string
		wantContents map[string]string
		wantContext  bool
	}{
		{
			name:    "apply_patch formats go and markdown files",
			payload: codexPostToolUsePayload("/repo", json.RawMessage(`{"command":"*** Begin Patch\n*** Update File: src/main.go\n@@\n-old\n+new\n*** Add File: docs/readme.md\n+text\n*** End Patch\n"}`)),
			files: map[string]string{
				"/repo/src/main.go":    unformattedGo,
				"/repo/docs/readme.md": unformattedMd,
			},
			wantContents: map[string]string{
				"/repo/src/main.go":    formattedGo,
				"/repo/docs/readme.md": formattedMd,
			},
			wantContext: true,
		},
		{
			name:    "direct file_path is supported",
			payload: codexPostToolUsePayload("", json.RawMessage(`{"file_path":"/src/main.go"}`)),
			files: map[string]string{
				"/src/main.go": unformattedGo,
			},
			wantContents: map[string]string{
				"/src/main.go": formattedGo,
			},
			wantContext: true,
		},
		{
			name:    "ignored markdown file unchanged",
			payload: codexPostToolUsePayload("/repo", json.RawMessage(`{"command":"*** Begin Patch\n*** Update File: docs/themes/shortcode.md\n@@\n-old\n+new\n*** End Patch\n"}`)),
			files: map[string]string{
				"/repo/docs/themes/shortcode.md": unformattedMd,
			},
			extraFiles: map[string]string{
				"/repo/docs/.prettierignore": "themes/\n",
			},
			wantContents: map[string]string{
				"/repo/docs/themes/shortcode.md": unformattedMd,
			},
		},
		{
			name:    "unsupported extension unchanged",
			payload: codexPostToolUsePayload("/repo", json.RawMessage(`{"command":"*** Begin Patch\n*** Update File: src/script.py\n@@\n-old\n+new\n*** End Patch\n"}`)),
			files: map[string]string{
				"/repo/src/script.py": "print('hello')\n",
			},
			wantContents: map[string]string{
				"/repo/src/script.py": "print('hello')\n",
			},
		},
		{
			name:    "missing path",
			payload: codexPostToolUsePayload("/repo", json.RawMessage(`{"command":"ls"}`)),
		},
		{
			name:    "deleted path skipped",
			payload: codexPostToolUsePayload("/repo", json.RawMessage(`{"command":"*** Begin Patch\n*** Delete File: src/main.go\n*** End Patch\n"}`)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			for path, content := range tt.files {
				if err := afero.WriteFile(fs, path, []byte(content), 0o644); err != nil {
					t.Fatalf("seeding file: %v", err)
				}
			}
			for path, content := range tt.extraFiles {
				if err := afero.WriteFile(fs, path, []byte(content), 0o644); err != nil {
					t.Fatalf("seeding extra file: %v", err)
				}
			}

			var stdout bytes.Buffer
			cmd := &postToolUseCmd{}
			if err := cmd.Run(strings.NewReader(tt.payload), &stdout, fs, logging.Discard); err != nil {
				t.Fatalf("Run() error: %v", err)
			}

			var out cx.PostToolUseOutput
			if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
				t.Fatalf("invalid output JSON: %v\nraw: %s", err, stdout.String())
			}
			hasContext := out.HookSpecificOutput != nil && out.HookSpecificOutput.AdditionalContext != nil
			if hasContext != tt.wantContext {
				t.Errorf("additional context present = %v, want %v", hasContext, tt.wantContext)
			}
			for path, want := range tt.wantContents {
				got, err := afero.ReadFile(fs, path)
				if err != nil {
					t.Fatalf("reading result file: %v", err)
				}
				if string(got) != want {
					t.Errorf("%s content mismatch:\ngot:  %q\nwant: %q", path, got, want)
				}
			}
		})
	}
}

func TestHandlers_InvalidJSON(t *testing.T) {
	handlers := []struct {
		name string
		run  runHandler
	}{
		{"session-start", stubHandler(new(sessionStartCmd).Run)},
		{"pre-tool-use", stubHandler(new(preToolUseCmd).Run)},
		{"permission-request", stubHandler(new(permissionRequestCmd).Run)},
		{"stop", stubHandler(new(stopCmd).Run)},
	}

	for _, tt := range handlers {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			if err := tt.run(strings.NewReader("not json"), &stdout); err == nil {
				t.Fatal("expected error for invalid JSON, got nil")
			}
		})
	}
}
