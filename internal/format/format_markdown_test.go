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

func TestFormatMarkdownFrontMatter(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		want    string
		wantNil bool
	}{
		{
			name: "YAML front matter is preserved",
			src:  "---\n# keep this comment\nname: thing\ntags:\n  - alpha\n  - beta\nmetadata:\n  type: guide\n---\n#  Title\n\nbody\n",
			want: "---\n# keep this comment\nname: thing\ntags:\n  - alpha\n  - beta\nmetadata:\n  type: guide\n---\n# Title\n\nbody\n",
		},
		{
			name: "TOML front matter is preserved",
			src:  "+++\ntitle = \"Thing\"\n[params]\n  author = \"Ada\"\n+++\n#  Title\n\nbody\n",
			want: "+++\ntitle = \"Thing\"\n[params]\n  author = \"Ada\"\n+++\n# Title\n\nbody\n",
		},
		{
			name: "JSON front matter is preserved",
			src:  "{\n  \"title\": \"Thing\",\n  \"params\": { \"author\": \"Ada\" }\n}\n#  Title\n\nbody\n",
			want: "{\n  \"title\": \"Thing\",\n  \"params\": { \"author\": \"Ada\" }\n}\n# Title\n\nbody\n",
		},
		{
			name: "leading whitespace and byte order mark are preserved",
			src:  "\ufeff\n  \n---\ntitle: Thing\n---\n#  Title\n",
			want: "\ufeff\n  \n---\ntitle: Thing\n---\n# Title\n",
		},
		{
			name: "CRLF front matter is preserved",
			src:  "---\r\ntitle: Thing\r\n---\r\n#  Title\r\n",
			want: "---\r\ntitle: Thing\r\n---\r\n# Title\n",
		},
		{
			name:    "unterminated YAML front matter is declined",
			src:     "---\ntitle: Thing\n#  Title\n",
			wantNil: true,
		},
		{
			name:    "unterminated TOML front matter is declined",
			src:     "+++\ntitle = \"Thing\"\n#  Title\n",
			wantNil: true,
		},
		{
			name:    "invalid JSON front matter is declined",
			src:     "{\n  \"title\": \n}\n#  Title\n",
			wantNil: true,
		},
		{
			name:    "JSON front matter without a line boundary is declined",
			src:     "{\"title\": \"Thing\"}# Title\n",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatMarkdown([]byte(tt.src))
			if err != nil {
				t.Fatalf("FormatMarkdown() error: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("FormatMarkdown() = %q, want nil", got)
				}
				return
			}
			if want := []byte(tt.want); !bytes.Equal(got, want) {
				t.Errorf("FormatMarkdown() = %q, want %q", got, want)
			}

			gotAgain, err := FormatMarkdown(got)
			if err != nil {
				t.Fatalf("second FormatMarkdown() error: %v", err)
			}
			if !bytes.Equal(gotAgain, got) {
				t.Errorf("second FormatMarkdown() = %q, want %q", gotAgain, got)
			}
		})
	}
}
