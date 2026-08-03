//go:build goexperiment.jsonv2

package claude

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func BenchmarkRenderContextBar(b *testing.B) {
	pct := 47.3
	for b.Loop() {
		_ = renderContextBar(&pct, false)
	}
}

func TestRenderContextBar(t *testing.T) {
	tests := []struct {
		name      string
		pct       *float64
		short     bool
		used      int
		available int
	}{
		{name: "nil", available: 20},
		{name: "half", pct: new(50.0), used: 10, available: 10},
		{name: "short_half", pct: new(50.0), short: true, used: 5, available: 5},
		{name: "below_zero", pct: new(-5.0), available: 20},
		{name: "above_one_hundred", pct: new(105.0), used: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderContextBar(tt.pct, tt.short)
			if count := strings.Count(got, "🟣"); count != tt.used {
				t.Errorf("used circles = %d, want %d", count, tt.used)
			}
			if count := strings.Count(got, "🟢"); count != tt.available {
				t.Errorf("unused circles = %d, want %d", count, tt.available)
			}
			if strings.Contains(got, "%") {
				t.Errorf("context display must not include a percentage: %q", got)
			}
		})
	}
}

func TestStatusLineLogsFullInputOnlyAtDebug(t *testing.T) {
	payload := `{"cwd":"/tmp","workspace":{"current_dir":"/tmp"},"context_window":{"used_percentage":50},"future_field":"preserved"}`

	var debugLog bytes.Buffer
	debug := slog.New(slog.NewJSONHandler(&debugLog, &slog.HandlerOptions{Level: slog.LevelDebug}))
	if err := (&statusLineCmd{}).Run(strings.NewReader(payload), io.Discard, debug); err != nil {
		t.Fatalf("Run() with debug logger: %v", err)
	}
	if got := inputPayloadFromLog(t, debugLog.Bytes()); got != payload {
		t.Fatalf("logged payload = %q, want %q", got, payload)
	}

	var infoLog bytes.Buffer
	info := slog.New(slog.NewJSONHandler(&infoLog, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := (&statusLineCmd{}).Run(strings.NewReader(payload), io.Discard, info); err != nil {
		t.Fatalf("Run() with info logger: %v", err)
	}
	if strings.Contains(infoLog.String(), "future_field") {
		t.Fatalf("info log unexpectedly contains full payload: %s", infoLog.String())
	}
}

func inputPayloadFromLog(t *testing.T, logs []byte) string {
	t.Helper()
	for _, line := range bytes.Split(bytes.TrimSpace(logs), []byte{'\n'}) {
		var record struct {
			Message string `json:"msg"`
			Payload string `json:"payload"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("unmarshal log record: %v", err)
		}
		if record.Message == "input" {
			return record.Payload
		}
	}
	t.Fatal("input log record not found")
	return ""
}

func TestWriteGitIndicators_Clean(t *testing.T) {
	var b strings.Builder
	writeGitIndicators(&b, 0, 0, 0, 0)
	if !strings.Contains(b.String(), "✓") {
		t.Errorf("expected ✓ for clean tree, got %q", b.String())
	}
}

func TestWriteGitIndicators_Dirty(t *testing.T) {
	var b strings.Builder
	writeGitIndicators(&b, 2, 3, 0, 0)
	s := b.String()
	if !strings.Contains(s, "+2") {
		t.Errorf("expected +2 staged, got %q", s)
	}
	if !strings.Contains(s, "~3") {
		t.Errorf("expected ~3 unstaged, got %q", s)
	}
	if strings.Contains(s, "✓") {
		t.Error("should not show ✓ when dirty")
	}
}

func TestWriteGitIndicators_AheadBehind(t *testing.T) {
	var b strings.Builder
	writeGitIndicators(&b, 0, 0, 3, 1)
	s := b.String()
	if !strings.Contains(s, "↑3") {
		t.Errorf("expected ↑3 ahead, got %q", s)
	}
	if !strings.Contains(s, "↓1") {
		t.Errorf("expected ↓1 behind, got %q", s)
	}
}

func TestShortModelName(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want string
	}{
		{"claude-sonnet-4-5-20250514", "S4.5"},
		{"claude-opus-4-6-v1", "O4.6"},
		{"eu.anthropic.claude-opus-4-6-v1", "O4.6"},
		{"claude-haiku-4-5-20251001", "H4.5"},
		{"eu.anthropic.claude-opus-4-7", "O4.7"},
		{"claude-opus-4-7[1m]", "O4.7-1M"},
		{"eu.anthropic.claude-opus-4-7[1m]", "O4.7-1M"},
		{"anthropic.claude-opus-4-7[1m]", "O4.7-1M"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			got := shortModelName(tc.id)
			if got != tc.want {
				t.Errorf("shortModelName(%q) = %q, want %q", tc.id, got, tc.want)
			}
		})
	}
}

func TestCompactPath(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
		max  int
		want string
	}{
		{"fits", "scut/cmd", 25, "scut/cmd"},
		{"collapse_one", "scut/internal/cmd/claude", 20, "scut/i/cmd/claude"},
		{"collapse_all", "scut/internal/cmd/claude", 19, "scut/i/cmd/claude"},
		{"elision", "scut/internal/cmd/claude", 15, "scut/i/c/claude"},
		{"two_segments", "scut/claude", 12, "scut/claude"},
		{"single", "scut", 6, "scut"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := compactPath(tc.path, tc.max)
			if got != tc.want {
				t.Errorf("compactPath(%q, %d) = %q, want %q", tc.path, tc.max, got, tc.want)
			}
			if runeLen := len([]rune(got)); runeLen > tc.max {
				t.Errorf("result display width %d exceeds max %d", runeLen, tc.max)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	for _, tc := range []struct {
		s    string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{"getting-started", 10, "getting-s…"},
		{"feat/very-long-branch-name", 20, "feat/very-long-bran…"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			got := truncate(tc.s, tc.max)
			if got != tc.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tc.s, tc.max, got, tc.want)
			}
		})
	}
}

func TestTildeRelative(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	for _, tc := range []struct {
		name string
		cwd  string
		want string
	}{
		{"subdir", home + "/projects/foo", "~/projects/foo"},
		{"home_root", home, "~/."},
		{"outside_home", "/tmp", "tmp"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tildeRelative(tc.cwd)
			if got != tc.want {
				t.Errorf("tildeRelative(%q) = %q, want %q", tc.cwd, got, tc.want)
			}
		})
	}
}
