//go:build goexperiment.jsonv2

package config

import "testing"

func TestOwns(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		// Exact match.
		{"botctrl", true},
		// Space prefix.
		{"botctrl claude hook post-tool-use", true},
		// Tab prefix.
		{"botctrl\tclaude hook post-tool-use", true},
		// Leading whitespace is stripped.
		{"  botctrl claude hook session-start", true},
		{"\t botctrl claude status-line", true},
		// Not botctrl.
		{"not-botctrl ...", false},
		// Token boundary: must not match "botctrlsomething".
		{"botctrlsomething", false},
		{"botctrlx claude hook post-tool-use", false},
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
