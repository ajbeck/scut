//go:build goexperiment.jsonv2

package claude

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

// runHandler normalizes the two handler signatures into one callable.
// Stub handlers take (io.Reader, io.Writer); postToolUseCmd also takes afero.Fs.
type runHandler func(io.Reader, io.Writer) error

func stubHandler(fn func(io.Reader, io.Writer) error) runHandler {
	return fn
}

func fsHandler(fn func(io.Reader, io.Writer, afero.Fs) error) runHandler {
	return func(r io.Reader, w io.Writer) error {
		return fn(r, w, afero.NewMemMapFs())
	}
}

// minimalInput is a valid JSON object for any hook input type.
// All fields have zero values; the decoders accept this without error.
var minimalInput = `{"session_id":"test","hook_event_name":"test"}`

func TestHandlers_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		run  runHandler
	}{
		{"session-start", stubHandler(new(sessionStartCmd).Run)},
		{"session-end", stubHandler(new(sessionEndCmd).Run)},
		{"instructions-loaded", stubHandler(new(instructionsLoadedCmd).Run)},
		{"user-prompt-submit", stubHandler(new(userPromptSubmitCmd).Run)},
		{"pre-tool-use", stubHandler(new(preToolUseCmd).Run)},
		{"post-tool-use", fsHandler(new(postToolUseCmd).Run)},
		{"post-tool-use-failure", stubHandler(new(postToolUseFailureCmd).Run)},
		{"permission-request", stubHandler(new(permissionRequestCmd).Run)},
		{"notification", stubHandler(new(notificationCmd).Run)},
		{"subagent-start", stubHandler(new(subagentStartCmd).Run)},
		{"subagent-stop", stubHandler(new(subagentStopCmd).Run)},
		{"stop", stubHandler(new(stopCmd).Run)},
		{"stop-failure", stubHandler(new(stopFailureCmd).Run)},
		{"task-created", stubHandler(new(taskCreatedCmd).Run)},
		{"task-completed", stubHandler(new(taskCompletedCmd).Run)},
		{"teammate-idle", stubHandler(new(teammateIdleCmd).Run)},
		{"config-change", stubHandler(new(configChangeCmd).Run)},
		{"cwd-changed", stubHandler(new(cwdChangedCmd).Run)},
		{"file-changed", stubHandler(new(fileChangedCmd).Run)},
		{"worktree-create", stubHandler(new(worktreeCreateCmd).Run)},
		{"worktree-remove", stubHandler(new(worktreeRemoveCmd).Run)},
		{"pre-compact", stubHandler(new(preCompactCmd).Run)},
		{"post-compact", stubHandler(new(postCompactCmd).Run)},
		{"elicitation", stubHandler(new(elicitationCmd).Run)},
		{"elicitation-result", stubHandler(new(elicitationResultCmd).Run)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdin := strings.NewReader(minimalInput)
			var stdout bytes.Buffer

			if err := tt.run(stdin, &stdout); err != nil {
				t.Fatalf("Run() error: %v", err)
			}

			// Verify output is valid JSON.
			if !json.Valid(stdout.Bytes()) {
				t.Fatalf("output is not valid JSON: %s", stdout.String())
			}
		})
	}
}

func TestHandlers_InvalidJSON(t *testing.T) {
	// All handlers should return an error when stdin contains invalid JSON.
	handlers := []struct {
		name string
		run  runHandler
	}{
		{"session-start", stubHandler(new(sessionStartCmd).Run)},
		{"post-tool-use", fsHandler(new(postToolUseCmd).Run)},
		{"pre-tool-use", stubHandler(new(preToolUseCmd).Run)},
		{"permission-request", stubHandler(new(permissionRequestCmd).Run)},
		{"elicitation", stubHandler(new(elicitationCmd).Run)},
	}

	for _, tt := range handlers {
		t.Run(tt.name, func(t *testing.T) {
			stdin := strings.NewReader("not json")
			var stdout bytes.Buffer

			if err := tt.run(stdin, &stdout); err == nil {
				t.Fatal("expected error for invalid JSON, got nil")
			}
		})
	}
}

func TestWriteJSON(t *testing.T) {
	t.Run("indented output", func(t *testing.T) {
		var buf bytes.Buffer
		input := struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}{"alice", 30}

		if err := writeJSON(&buf, input); err != nil {
			t.Fatalf("writeJSON error: %v", err)
		}

		want := "{\n  \"name\": \"alice\",\n  \"age\": 30\n}\n"
		if buf.String() != want {
			t.Errorf("got %q, want %q", buf.String(), want)
		}
	})

	t.Run("empty struct", func(t *testing.T) {
		var buf bytes.Buffer
		if err := writeJSON(&buf, struct{}{}); err != nil {
			t.Fatalf("writeJSON error: %v", err)
		}
		if buf.String() != "{}\n" {
			t.Errorf("got %q, want %q", buf.String(), "{}\n")
		}
	})
}
