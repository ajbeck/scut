//go:build goexperiment.jsonv2

package config

import "testing"

func TestOwns(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		// Exact match.
		{"scut", true},
		// Space prefix.
		{"scut claude hook post-tool-use", true},
		// Tab prefix.
		{"scut\tclaude hook post-tool-use", true},
		// Leading whitespace is stripped.
		{"  scut claude hook session-start", true},
		{"\t scut claude status-line", true},
		// Not scut.
		{"not-scut ...", false},
		// Token boundary: must not match "scutsomething".
		{"scutsomething", false},
		{"scutx claude hook post-tool-use", false},
		// Empty string.
		{"", false},
		// Other tools.
		{"gofmt -w .", false},
		{"bash -c echo", false},
	}

	for _, tt := range tests {
		got := owns(tt.command)
		if got != tt.want {
			t.Errorf("owns(%q) = %v, want %v", tt.command, got, tt.want)
		}
	}
}
