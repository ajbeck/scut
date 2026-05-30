//go:build goexperiment.jsonv2

package hook

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/ajbeck/scut/internal/logging"
)

type runHandler func(io.Reader, io.Writer) error

func stubHandler(fn func(io.Reader, io.Writer, *slog.Logger) error) runHandler {
	return func(r io.Reader, w io.Writer) error {
		return fn(r, w, logging.Discard)
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
		{"post-tool-use", stubHandler(new(postToolUseCmd).Run)},
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
