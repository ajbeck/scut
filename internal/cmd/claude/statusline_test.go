//go:build goexperiment.jsonv2

package claude

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func BenchmarkRenderContextBar(b *testing.B) {
	pct := 47.3
	for b.Loop() {
		_ = renderContextBar(&pct)
	}
}

func BenchmarkRenderContextBar_Nil(b *testing.B) {
	for b.Loop() {
		_ = renderContextBar(nil)
	}
}

func BenchmarkRenderContextBar_Thresholds(b *testing.B) {
	thresholds := []float64{25.0, 75.0, 83.0, 95.0}
	for _, pct := range thresholds {
		pct := pct
		b.Run(fmt.Sprintf("%d", int(pct)), func(b *testing.B) {
			for b.Loop() {
				_ = renderContextBar(&pct)
			}
		})
	}
}

func TestRenderContextBar_Nil(t *testing.T) {
	got := renderContextBar(nil)
	if got != nullBar {
		t.Errorf("nil bar mismatch:\n  got:  %q\n  want: %q", got, nullBar)
	}
}

func TestRenderContextBar_MarkerAlwaysPresent(t *testing.T) {
	// The compaction marker │ must appear at every percentage and at nil.
	for p := range 101 {
		pct := float64(p)
		result := renderContextBar(&pct)
		if !strings.Contains(result, "│") {
			t.Errorf("marker missing at %d%%", p)
		}
	}
	if !strings.Contains(renderContextBar(nil), "│") {
		t.Error("marker missing in nil bar")
	}
}

func TestRenderContextBar_Boundaries(t *testing.T) {
	// Both filled and unfilled use █ (distinguished by ANSI colour), so we
	// count total █ (should always equal fillArea minus any ▌) and ▌ separately.
	for _, tc := range []struct {
		pct      float64
		wantHalf int
	}{
		{0, 0},
		{100, 0},
		{50, 1}, // 50*19*2/100=19 halves, full=9, half=1
	} {
		t.Run(fmt.Sprintf("%d%%", int(tc.pct)), func(t *testing.T) {
			pct := tc.pct
			result := renderContextBar(&pct)
			totalBlocks := strings.Count(result, "█")
			halfCount := strings.Count(result, "▌")
			wantBlocks := fillArea - halfCount
			if totalBlocks != wantBlocks {
				t.Errorf("total █ blocks: got %d, want %d", totalBlocks, wantBlocks)
			}
			if halfCount != tc.wantHalf {
				t.Errorf("half blocks: got %d, want %d", halfCount, tc.wantHalf)
			}
		})
	}
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
		{"fits", "botctrl/cmd", 25, "botctrl/cmd"},
		{"collapse_one", "botctrl/internal/cmd/claude", 20, "botctrl/i/cmd/claude"},
		{"collapse_all", "botctrl/internal/cmd/claude", 19, "botctrl/i/c/claude"},
		{"elision", "botctrl/internal/cmd/claude", 15, "botctrl/…/clau…"},
		{"two_segments", "botctrl/claude", 12, "botctrl/cla…"},
		{"single", "botctrl", 6, "botct…"},
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

func TestRenderContextBar_AllPercentages(t *testing.T) {
	// Verify every percentage produces non-empty output and the total block
	// count (█ + ▌) always equals fillArea (both filled and unfilled use █).
	for p := range 101 {
		pct := float64(p)
		result := renderContextBar(&pct)
		if result == "" {
			t.Fatalf("empty result at %d%%", p)
		}
		blocks := strings.Count(result, "█") + strings.Count(result, "▌")
		if blocks != fillArea {
			t.Errorf("total blocks at %d%%: got %d, want %d", p, blocks, fillArea)
		}
	}
}

func TestRenderContextBar_ThresholdBoundary(t *testing.T) {
	// The bar must use warning colour at compactionThreshold-1 and error
	// colour at compactionThreshold. We verify by checking that the two
	// renders produce different ANSI output (the colour codes differ).
	below := float64(compactionThreshold - 1)
	at := float64(compactionThreshold)
	resultBelow := renderContextBar(&below)
	resultAt := renderContextBar(&at)
	if resultBelow == resultAt {
		t.Errorf("bar at %d%% and %d%% should differ in colour but are identical", compactionThreshold-1, compactionThreshold)
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
