//go:build goexperiment.jsonv2

package claudecode

import (
	"encoding/json"
	"testing"
)

func TestPostToolUseOutput_OmitEmpty(t *testing.T) {
	t.Run("zero value omits all fields", func(t *testing.T) {
		out := PostToolUseOutput{}
		data, err := json.Marshal(out)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if string(data) != "{}" {
			t.Errorf("got %s, want {}", data)
		}
	})

	t.Run("set fields are present", func(t *testing.T) {
		out := PostToolUseOutput{
			Decision:          new(DecisionBlock),
			Reason:            new("bad output"),
			AdditionalContext: new("extra info"),
			HookSpecificOutput: &PostToolUseHookOutput{
				HookEventName:     EventPostToolUse,
				AdditionalContext: new("nested info"),
			},
		}
		data, err := json.Marshal(out)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if m["decision"] != "block" {
			t.Errorf("decision = %v, want %q", m["decision"], "block")
		}
		if m["reason"] != "bad output" {
			t.Errorf("reason = %v, want %q", m["reason"], "bad output")
		}
		if m["additionalContext"] != "extra info" {
			t.Errorf("additionalContext = %v, want %q", m["additionalContext"], "extra info")
		}
		hook, ok := m["hookSpecificOutput"].(map[string]any)
		if !ok {
			t.Fatalf("hookSpecificOutput missing or wrong type: %v", m["hookSpecificOutput"])
		}
		if hook["hookEventName"] != "PostToolUse" {
			t.Errorf("hookEventName = %v, want %q", hook["hookEventName"], "PostToolUse")
		}
		if hook["additionalContext"] != "nested info" {
			t.Errorf("hook additionalContext = %v, want %q", hook["additionalContext"], "nested info")
		}
	})
}

func TestPreToolUseOutput_NestedHookOutput(t *testing.T) {
	out := PreToolUseOutput{
		HookSpecificOutput: PreToolUseHookOutput{
			HookEventName:      EventPreToolUse,
			PermissionDecision: PermissionAllow,
		},
	}
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	hook, ok := m["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("hookSpecificOutput missing or wrong type: %v", m["hookSpecificOutput"])
	}
	if hook["hookEventName"] != "PreToolUse" {
		t.Errorf("hookEventName = %v, want %q", hook["hookEventName"], "PreToolUse")
	}
	if hook["permissionDecision"] != "allow" {
		t.Errorf("permissionDecision = %v, want %q", hook["permissionDecision"], "allow")
	}
}

func TestBaseOutput_OmitEmpty(t *testing.T) {
	t.Run("zero value omits all fields", func(t *testing.T) {
		out := BaseOutput{}
		data, err := json.Marshal(out)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if string(data) != "{}" {
			t.Errorf("got %s, want {}", data)
		}
	})

	t.Run("bool pointer fields", func(t *testing.T) {
		out := BaseOutput{
			Continue:       new(false),
			SuppressOutput: new(true),
		}
		data, err := json.Marshal(out)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if m["continue"] != false {
			t.Errorf("continue = %v, want false", m["continue"])
		}
		if m["suppressOutput"] != true {
			t.Errorf("suppressOutput = %v, want true", m["suppressOutput"])
		}
	})
}

func TestPostToolUseInput_RoundTrip(t *testing.T) {
	raw := `{
		"session_id": "abc",
		"hook_event_name": "PostToolUse",
		"effort": {"level": "high"},
		"tool_name": "Write",
		"tool_use_id": "tu_123",
		"tool_input": {"file_path": "/x.go", "content": "package main"},
		"tool_response": {"status": "ok"}
	}`

	var in PostToolUseInput
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if in.SessionID != "abc" {
		t.Errorf("SessionID = %q, want %q", in.SessionID, "abc")
	}
	if in.ToolName != "Write" {
		t.Errorf("ToolName = %q, want %q", in.ToolName, "Write")
	}
	if in.Effort == nil || in.Effort.Level != EffortHigh {
		t.Errorf("Effort = %#v, want level %q", in.Effort, EffortHigh)
	}
	if in.FilePath() != "/x.go" {
		t.Errorf("FilePath() = %q, want %q", in.FilePath(), "/x.go")
	}
	// RawMessage fields preserve the original JSON.
	if len(in.ToolInput) == 0 {
		t.Error("ToolInput is empty after unmarshal")
	}
	if len(in.ToolResponse) == 0 {
		t.Error("ToolResponse is empty after unmarshal")
	}
}

func TestPostToolBatchInput_EffortObject(t *testing.T) {
	raw := `{
		"session_id": "abc",
		"hook_event_name": "PostToolBatch",
		"effort": {"level": "medium"},
		"tool_calls": [
			{
				"tool_name": "Read",
				"tool_use_id": "tu_123",
				"tool_input": {"file_path": "/x.go"},
				"tool_response": "     1\tpackage main"
			}
		]
	}`

	var in PostToolBatchInput
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if in.Effort == nil || in.Effort.Level != EffortMedium {
		t.Errorf("Effort = %#v, want level %q", in.Effort, EffortMedium)
	}
	if len(in.ToolCalls) != 1 {
		t.Fatalf("ToolCalls length = %d, want 1", len(in.ToolCalls))
	}
	if in.ToolCalls[0].ToolName != "Read" {
		t.Errorf("ToolCalls[0].ToolName = %q, want %q", in.ToolCalls[0].ToolName, "Read")
	}
	if !json.Valid(in.ToolCalls[0].ToolResponse) {
		t.Errorf("ToolCalls[0].ToolResponse is invalid JSON: %s", in.ToolCalls[0].ToolResponse)
	}
}
