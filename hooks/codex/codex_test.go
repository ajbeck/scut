//go:build goexperiment.jsonv2

package codex

import (
	"encoding/json"
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
