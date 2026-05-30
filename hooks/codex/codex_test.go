//go:build goexperiment.jsonv2

package codex

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestPostToolUseInput_RoundTrip(t *testing.T) {
	data := []byte(`{
		"session_id": "abc123",
		"transcript_path": "/tmp/transcript.jsonl",
		"cwd": "/repo",
		"hook_event_name": "PostToolUse",
		"model": "gpt-5.1-codex-max",
		"permission_mode": "default",
		"turn_id": "turn-1",
		"tool_name": "apply_patch",
		"tool_use_id": "toolu_1",
		"tool_input": {"command": "*** Begin Patch\n*** End Patch\n"},
		"tool_response": {"ok": true}
	}`)

	var in PostToolUseInput
	if err := json.Unmarshal(data, &in); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if in.HookEventName != EventPostToolUse {
		t.Errorf("HookEventName = %q, want %q", in.HookEventName, EventPostToolUse)
	}
	if in.ToolName != "apply_patch" {
		t.Errorf("ToolName = %q, want apply_patch", in.ToolName)
	}
	if !json.Valid(in.ToolInput) {
		t.Fatalf("ToolInput is not valid JSON: %s", in.ToolInput)
	}
}

func TestPreToolUseOutput_NestedHookOutput(t *testing.T) {
	out := PreToolUseOutput{
		HookSpecificOutput: &PreToolUseHookOutput{
			HookEventName:            EventPreToolUse,
			PermissionDecision:       PermissionDeny,
			PermissionDecisionReason: new("blocked"),
		},
	}

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	hook, ok := m["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("hookSpecificOutput missing or wrong type: %v", m["hookSpecificOutput"])
	}
	if hook["hookEventName"] != "PreToolUse" {
		t.Errorf("hookEventName = %v, want PreToolUse", hook["hookEventName"])
	}
	if hook["permissionDecision"] != "deny" {
		t.Errorf("permissionDecision = %v, want deny", hook["permissionDecision"])
	}
}

func TestPostToolUseInput_FilePaths(t *testing.T) {
	tests := []struct {
		name      string
		cwd       string
		toolInput json.RawMessage
		want      []string
	}{
		{
			name:      "direct file_path",
			toolInput: json.RawMessage(`{"file_path":"/src/main.go"}`),
			want:      []string{"/src/main.go"},
		},
		{
			name:      "apply_patch add update delete",
			cwd:       "/repo",
			toolInput: json.RawMessage(`{"command":"*** Begin Patch\n*** Add File: docs/new.md\n+#  Hi\n*** Update File: src/main.go\n@@\n-old\n+new\n*** Delete File: old.txt\n*** End Patch\n"}`),
			want:      []string{"/repo/docs/new.md", "/repo/src/main.go"},
		},
		{
			name:      "apply_patch move uses destination",
			cwd:       "/repo",
			toolInput: json.RawMessage(`{"patch":"*** Begin Patch\n*** Update File: docs/old.md\n*** Move to: docs/new.md\n@@\n-old\n+new\n*** End Patch\n"}`),
			want:      []string{"/repo/docs/new.md"},
		},
		{
			name:      "duplicates removed",
			cwd:       "/repo",
			toolInput: json.RawMessage(`{"file_path":"src/main.go","command":"*** Begin Patch\n*** Update File: src/main.go\n@@\n-old\n+new\n*** End Patch\n"}`),
			want:      []string{"/repo/src/main.go"},
		},
		{
			name:      "missing path",
			toolInput: json.RawMessage(`{"command":"ls"}`),
		},
		{
			name:      "invalid json",
			toolInput: json.RawMessage(`{`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := &PostToolUseInput{
				TurnInput: TurnInput{Input: Input{CWD: tt.cwd}},
				ToolInput: tt.toolInput,
			}
			if got := in.FilePaths(); !slices.Equal(got, tt.want) {
				t.Errorf("FilePaths() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
