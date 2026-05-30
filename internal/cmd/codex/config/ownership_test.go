//go:build goexperiment.jsonv2

package config

import "testing"

func TestOwns(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{
			name:    "plain generated hook",
			command: "scut codex hook post-tool-use",
			want:    true,
		},
		{
			name:    "baked log generated hook",
			command: "scut codex --log hook post-tool-use",
			want:    true,
		},
		{
			name:    "baked log level generated hook",
			command: "scut codex --log-level=debug hook post-tool-use",
			want:    true,
		},
		{
			name:    "log level non hook",
			command: "scut codex --log-level=debug status",
			want:    false,
		},
		{
			name:    "foreign command",
			command: "other codex hook post-tool-use",
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := owns(tt.command); got != tt.want {
				t.Errorf("owns(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}
