package formatignore

import (
	"testing"

	"github.com/spf13/afero"
)

func TestMatchPath(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		dirs    []string
		path    string
		want    bool
		wantErr bool
	}{
		{
			name: "prettierignore_matches_directory_pattern",
			dirs: []string{"/repo/.git", "/repo/docs/themes"},
			files: map[string]string{
				"/repo/.prettierignore":          "docs/themes/\n",
				"/repo/docs/themes/shortcode.md": "#  Hello\n",
			},
			path: "/repo/docs/themes/shortcode.md",
			want: true,
		},
		{
			name: "prettierignore_matches_double_star_pattern",
			dirs: []string{"/repo/.git", "/repo/docs"},
			files: map[string]string{
				"/repo/.prettierignore": "**/*.tmpl\n",
				"/repo/docs/page.tmpl":  "template\n",
			},
			path: "/repo/docs/page.tmpl",
			want: true,
		},
		{
			name: "scutignore_overrides_prettierignore",
			dirs: []string{"/repo/.git", "/repo/docs"},
			files: map[string]string{
				"/repo/.prettierignore": "docs/\n",
				"/repo/.scutignore":     "!docs/readme.md\n",
				"/repo/docs/readme.md":  "#  Hello\n",
			},
			path: "/repo/docs/readme.md",
			want: false,
		},
		{
			name: "nearest_root_wins",
			dirs: []string{"/repo/.git", "/repo/docs"},
			files: map[string]string{
				"/repo/.prettierignore":      "docs/\n",
				"/repo/docs/.prettierignore": "!readme.md\n",
				"/repo/docs/readme.md":       "#  Hello\n",
			},
			path: "/repo/docs/readme.md",
			want: false,
		},
		{
			name: "no_root_does_not_ignore",
			dirs: []string{"/tmp/docs"},
			files: map[string]string{
				"/tmp/docs/readme.md": "#  Hello\n",
			},
			path: "/tmp/docs/readme.md",
			want: false,
		},
		{
			name: "comment_and_blank_lines_do_not_match",
			dirs: []string{"/repo/.git", "/repo/docs"},
			files: map[string]string{
				"/repo/.prettierignore": "# docs/\n\n",
				"/repo/docs/readme.md":  "#  Hello\n",
			},
			path: "/repo/docs/readme.md",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			for _, dir := range tt.dirs {
				if err := fs.MkdirAll(dir, 0o755); err != nil {
					t.Fatalf("MkdirAll(%q): %v", dir, err)
				}
			}
			for path, content := range tt.files {
				if err := afero.WriteFile(fs, path, []byte(content), 0o644); err != nil {
					t.Fatalf("WriteFile(%q): %v", path, err)
				}
			}

			got, err := MatchPath(fs, tt.path, false)
			if (err != nil) != tt.wantErr {
				t.Fatalf("MatchPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("MatchPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
