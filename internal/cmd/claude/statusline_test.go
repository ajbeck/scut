//go:build goexperiment.jsonv2

package claude

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// contextBarString calls writeContextBar and returns the result as a string.
func contextBarString(pct *float64) string {
	var b strings.Builder
	writeContextBar(&b, pct)
	return b.String()
}

// writeContextBarMath is the runtime-math equivalent of writeContextBar.
// It computes the bar string on every call using strings.Repeat and partial
// block selection — the approach the hardcoded lookup table replaced.
func writeContextBarMath(b *strings.Builder, pct *float64) {
	if pct == nil {
		b.WriteString(nullBar)
		return
	}

	p := min(max(int(math.Round(*pct)), 0), 100)

	// Compute bar via arithmetic (the old approach).
	blocks := []string{" ", "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}
	const w = 10
	eighths := p * w * 8 / 100
	full := eighths / 8
	partial := eighths % 8
	empty := w - full
	if partial > 0 {
		empty--
	}

	bar := strings.Repeat("█", full)
	if partial > 0 {
		bar += blocks[partial]
	}
	bar += strings.Repeat("░", empty)

	label := bar + " " + strconv.Itoa(p) + "%"

	var style lipgloss.Style
	switch {
	case p >= 90:
		style = barStyleError
	case p >= 70:
		style = barStyleWarning
	default:
		style = barStyleMint
	}

	b.WriteString(style.Render(label))
}

func contextBarMathString(pct *float64) string {
	var b strings.Builder
	writeContextBarMath(&b, pct)
	return b.String()
}

func BenchmarkWriteContextBar_Lookup(b *testing.B) {
	pct := 47.3
	var buf strings.Builder
	for b.Loop() {
		buf.Reset()
		writeContextBar(&buf, &pct)
	}
}

func BenchmarkWriteContextBar_Math(b *testing.B) {
	pct := 47.3
	var buf strings.Builder
	for b.Loop() {
		buf.Reset()
		writeContextBarMath(&buf, &pct)
	}
}

func BenchmarkWriteContextBar_Nil(b *testing.B) {
	var buf strings.Builder
	for b.Loop() {
		buf.Reset()
		writeContextBar(&buf, nil)
	}
}

func BenchmarkWriteContextBar_Thresholds(b *testing.B) {
	thresholds := []float64{25.0, 75.0, 95.0}
	for _, pct := range thresholds {
		pct := pct
		b.Run(fmt.Sprintf("Lookup_%d", int(pct)), func(b *testing.B) {
			var buf strings.Builder
			for b.Loop() {
				buf.Reset()
				writeContextBar(&buf, &pct)
			}
		})
		b.Run(fmt.Sprintf("Math_%d", int(pct)), func(b *testing.B) {
			var buf strings.Builder
			for b.Loop() {
				buf.Reset()
				writeContextBarMath(&buf, &pct)
			}
		})
	}
}

func TestWriteContextBarMath_MatchesLookup(t *testing.T) {
	for p := range 101 {
		pct := float64(p)
		got := contextBarMathString(&pct)
		want := contextBarString(&pct)
		if got != want {
			t.Errorf("mismatch at %d%%:\n  math:   %q\n  lookup: %q", p, got, want)
		}
	}
}
