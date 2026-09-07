package claudecode

import (
	"encoding/json"
	"testing"
)

func TestStatusLineInputDecodesCurrentClaudeCodeSchema(t *testing.T) {
	const payload = `{
		"cwd":"/workspace/project",
		"session_id":"session-1",
		"session_name":"implement status line",
		"prompt_id":"prompt-1",
		"transcript_path":"/tmp/transcript.jsonl",
		"version":"2.1.216",
		"model":{"id":"claude-opus-5","display_name":"Opus"},
		"workspace":{
			"current_dir":"/workspace/project",
			"project_dir":"/workspace/project",
			"added_dirs":["/workspace/shared"],
			"git_worktree":"feature-status",
			"repo":{"host":"github.com","owner":"ajbeck","name":"scut"}
		},
		"cost":{"total_cost_usd":1.2,"total_duration_ms":2000,"total_api_duration_ms":1000,"total_lines_added":3,"total_lines_removed":2},
		"context_window":{"total_input_tokens":100,"total_output_tokens":20,"context_window_size":200000,"used_percentage":50,"remaining_percentage":50,"current_usage":{"input_tokens":70,"output_tokens":20,"cache_creation_input_tokens":10,"cache_read_input_tokens":20}},
		"exceeds_200k_tokens":false,
		"fast_mode":true,
		"effort":{"level":"high"},
		"thinking":{"enabled":true},
		"rate_limits":{"five_hour":{"used_percentage":20,"resets_at":1},"seven_day":{"used_percentage":30,"resets_at":2}},
		"output_style":{"name":"default"},
		"vim":{"mode":"NORMAL"},
		"agent":{"name":"reviewer"},
		"pr":{"number":42,"url":"https://github.com/ajbeck/scut/pull/42","review_state":"pending"},
		"worktree":{"name":"feature-status","path":"/workspace/worktree","branch":"feature-status","original_cwd":"/workspace/project","original_branch":"main"},
		"future_field":"ignored"
	}`

	var in StatusLineInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if in.SessionName == nil || *in.SessionName != "implement status line" {
		t.Fatalf("SessionName = %#v, want implement status line", in.SessionName)
	}
	if in.PromptID == nil || *in.PromptID != "prompt-1" {
		t.Fatalf("PromptID = %#v, want prompt-1", in.PromptID)
	}
	if got, want := in.Workspace.AddedDirs, []string{"/workspace/shared"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("AddedDirs = %#v, want %#v", got, want)
	}
	if in.Workspace.GitWorktree == nil || *in.Workspace.GitWorktree != "feature-status" {
		t.Fatalf("GitWorktree = %#v, want feature-status", in.Workspace.GitWorktree)
	}
	if in.Workspace.Repo == nil || in.Workspace.Repo.Name != "scut" {
		t.Fatalf("Repo = %#v, want scut", in.Workspace.Repo)
	}
	if !in.FastMode || in.Effort == nil || in.Effort.Level != "high" || in.Thinking == nil || !in.Thinking.Enabled {
		t.Fatalf("new session fields did not decode: %+v", in)
	}
	if in.PR == nil || in.PR.Number != 42 || in.PR.ReviewState == nil || *in.PR.ReviewState != "pending" {
		t.Fatalf("PR = %#v, want open PR metadata", in.PR)
	}
}
