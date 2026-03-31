package claudecode

import (
	"encoding/json"
	"testing"
)

func TestPostToolUseInput_FilePath(t *testing.T) {
	tests := []struct {
		name      string
		toolInput json.RawMessage
		want      string
	}{
		{
			name:      "valid path",
			toolInput: json.RawMessage(`{"file_path":"/src/main.go"}`),
			want:      "/src/main.go",
		},
		{
			name:      "missing field",
			toolInput: json.RawMessage(`{"command":"ls"}`),
			want:      "",
		},
		{
			name:      "nil input",
			toolInput: nil,
			want:      "",
		},
		{
			name:      "empty object",
			toolInput: json.RawMessage(`{}`),
			want:      "",
		},
		{
			name:      "non-string value",
			toolInput: json.RawMessage(`{"file_path":123}`),
			want:      "",
		},
		{
			name:      "empty string",
			toolInput: json.RawMessage(`{"file_path":""}`),
			want:      "",
		},
		{
			name:      "extra fields ignored",
			toolInput: json.RawMessage(`{"file_path":"/x.go","content":"stuff"}`),
			want:      "/x.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := &PostToolUseInput{ToolInput: tt.toolInput}
			if got := in.FilePath(); got != tt.want {
				t.Errorf("FilePath() = %q, want %q", got, tt.want)
			}
		})
	}
}
