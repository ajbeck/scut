package format

import (
	"bytes"
	"testing"
)

func TestFormatMarkdown(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		wantChange bool
		wantNil    bool
	}{
		{
			name:       "normalizes heading spacing",
			src:        "#  Hello\n\nworld\n",
			wantChange: true,
		},
		{
			name:       "already formatted",
			src:        "# Hello\n\nworld\n",
			wantChange: false,
		},
		{
			name:    "empty input",
			src:     "",
			wantNil: true,
		},
		{
			name:       "plain paragraph",
			src:        "Just some text.\n",
			wantChange: false,
		},
		{
			name:       "table column alignment",
			src:        "| Name | Age |\n| --- | --- |\n| Alice | 30 |\n| Bob | 7 |\n",
			wantChange: true,
		},
		{
			name:       "table preserved as table",
			src:        "| Name  | Age |\n| ----- | --- |\n| Alice | 30  |\n| Bob   | 7   |\n",
			wantChange: false,
		},
		{
			name:       "strikethrough roundtrip",
			src:        "Some ~~deleted~~ text.\n",
			wantChange: false,
		},
		{
			name:       "task checkbox roundtrip",
			src:        "- [x] done\n- [ ] todo\n",
			wantChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := []byte(tt.src)
			got, err := FormatMarkdown(src)
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
