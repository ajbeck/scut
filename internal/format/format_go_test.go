package format

import (
	"bytes"
	"testing"
)

func TestFormatGo(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		wantChange bool
		wantNil    bool
	}{
		{
			name:       "needs formatting",
			src:        "package main\n\nfunc main()  {}\n",
			wantChange: true,
		},
		{
			name:       "already formatted",
			src:        "package main\n\nfunc main() {}\n",
			wantChange: false,
		},
		{
			name:    "syntax error",
			src:     "func {{{",
			wantNil: true,
		},
		{
			name:       "empty input",
			src:        "",
			wantChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := []byte(tt.src)
			got, err := FormatGo(src)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Fatalf("want nil, got %q", got)
				}
				return
			}
			if got == nil {
				t.Fatal("got nil, want non-nil result")
			}
			changed := !bytes.Equal(src, got)
			if changed != tt.wantChange {
				t.Errorf("changed=%v, wantChange=%v\nsrc: %q\ngot: %q", changed, tt.wantChange, src, got)
			}
		})
	}
}