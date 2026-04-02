//go:build goexperiment.jsonv2

package claude

import (
	"fmt"
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
	thresholds := []float64{25.0, 75.0, 95.0}
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

func TestRenderContextBar_Boundaries(t *testing.T) {
	for _, tc := range []struct {
		pct       float64
		wantFull  int
		wantEmpty int
		wantHalf  int
	}{
		{0, 0, barWidth, 0},
		{100, barWidth, 0, 0},
		{50, 7, 7, 1}, // 50*15*2/100=15, full=7, half=1, empty=7
	} {
		t.Run(fmt.Sprintf("%d%%", int(tc.pct)), func(t *testing.T) {
			pct := tc.pct
			result := renderContextBar(&pct)
			fullCount := strings.Count(result, "█")
			emptyCount := strings.Count(result, "░")
			halfCount := strings.Count(result, "▌")
			if fullCount != tc.wantFull {
				t.Errorf("full blocks: got %d, want %d", fullCount, tc.wantFull)
			}
			if emptyCount != tc.wantEmpty {
				t.Errorf("empty blocks: got %d, want %d", emptyCount, tc.wantEmpty)
			}
			if halfCount != tc.wantHalf {
				t.Errorf("half blocks: got %d, want %d", halfCount, tc.wantHalf)
			}
		})
	}
}

func TestRenderContextBar_AllPercentages(t *testing.T) {
	// Verify every percentage produces output and filled count is monotonically non-decreasing.
	prevFull := 0
	for p := range 101 {
		pct := float64(p)
		result := renderContextBar(&pct)
		if result == "" {
			t.Fatalf("empty result at %d%%", p)
		}
		// Count full blocks + half blocks as "fill progress".
		fillCount := strings.Count(result, "█") + strings.Count(result, "▌")
		if fillCount < prevFull {
			t.Errorf("fill decreased from %d to %d at %d%%", prevFull, fillCount, p)
		}
		prevFull = fillCount
	}
}
