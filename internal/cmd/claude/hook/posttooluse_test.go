//go:build goexperiment.jsonv2

package hook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/ajbeck/scut/internal/logging"

	cc "github.com/ajbeck/scut/hooks/claudecode"
)

// toolInput builds a JSON tool_input payload with the given file_path.
func toolInput(path string) json.RawMessage {
	return json.RawMessage(`{"file_path":"` + path + `"}`)
}

// hookPayload builds a full PostToolUseInput JSON string.
func hookPayload(toolInput json.RawMessage) string {
	in := cc.PostToolUseInput{
		Input: cc.Input{
			SessionID:     "test-session",
			HookEventName: cc.EventPostToolUse,
		},
		ToolName:  "Write",
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
		name        string
		payload     string
		filePath    string
		fileContent string
		extraFiles  map[string]string
		wantContent string
		wantContext bool
	}{
		{
			name:        "go file gets formatted",
			payload:     hookPayload(toolInput("/src/main.go")),
			filePath:    "/src/main.go",
			fileContent: unformattedGo,
			wantContent: formattedGo,
			wantContext: true,
		},
		{
			name:        "md file gets formatted",
			payload:     hookPayload(toolInput("/docs/readme.md")),
			filePath:    "/docs/readme.md",
			fileContent: unformattedMd,
			wantContent: formattedMd,
			wantContext: true,
		},
		{
			name:        "mdx file gets formatted",
			payload:     hookPayload(toolInput("/docs/page.mdx")),
			filePath:    "/docs/page.mdx",
			fileContent: unformattedMd,
			wantContent: formattedMd,
			wantContext: true,
		},
		{
			name:        "ignored md file unchanged",
			payload:     hookPayload(toolInput("/docs/themes/shortcode.md")),
			filePath:    "/docs/themes/shortcode.md",
			fileContent: unformattedMd,
			extraFiles: map[string]string{
				"/docs/.prettierignore": "themes/\n",
			},
			wantContent: unformattedMd,
		},
		{
			name:        "already formatted go file unchanged",
			payload:     hookPayload(toolInput("/src/clean.go")),
			filePath:    "/src/clean.go",
			fileContent: formattedGo,
			wantContent: formattedGo,
		},
		{
			name:        "unknown extension unchanged",
			payload:     hookPayload(toolInput("/src/script.py")),
			filePath:    "/src/script.py",
			fileContent: "print('hello')\n",
			wantContent: "print('hello')\n",
		},
		{
			name:    "no file_path in tool_input",
			payload: hookPayload(json.RawMessage(`{"command":"ls"}`)),
		},
		{
			name:    "file does not exist",
			payload: hookPayload(toolInput("/gone.go")),
		},
		{
			name:    "empty tool_input",
			payload: hookPayload(json.RawMessage(`{}`)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()

			if tt.filePath != "" && tt.fileContent != "" {
				if err := afero.WriteFile(fs, tt.filePath, []byte(tt.fileContent), 0o644); err != nil {
					t.Fatalf("seeding file: %v", err)
				}
			}
			for path, content := range tt.extraFiles {
				if err := afero.WriteFile(fs, path, []byte(content), 0o644); err != nil {
					t.Fatalf("seeding extra file: %v", err)
				}
			}

			stdin := strings.NewReader(tt.payload)
			var stdout bytes.Buffer
			cmd := &postToolUseCmd{}

			if err := cmd.Run(stdin, &stdout, fs, logging.Discard); err != nil {
				t.Fatalf("Run() error: %v", err)
			}

			// Verify stdout is valid PostToolUseOutput JSON.
			var out cc.PostToolUseOutput
			if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
				t.Fatalf("invalid output JSON: %v\nraw: %s", err, stdout.String())
			}
			hasContext := out.HookSpecificOutput != nil && out.HookSpecificOutput.AdditionalContext != nil
			if hasContext != tt.wantContext {
				t.Errorf("additional context present = %v, want %v", hasContext, tt.wantContext)
			}

			// Verify file contents when a file was seeded.
			if tt.filePath != "" && tt.wantContent != "" {
				got, err := afero.ReadFile(fs, tt.filePath)
				if err != nil {
					t.Fatalf("reading result file: %v", err)
				}
				if string(got) != tt.wantContent {
					t.Errorf("file content mismatch:\ngot:  %q\nwant: %q", got, tt.wantContent)
				}
			}
		})
	}
}
