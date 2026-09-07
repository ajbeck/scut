package format

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	byteformat "github.com/ajbeck/scut/internal/format"
	"github.com/alecthomas/kong"
	"github.com/spf13/afero"
)

func TestFormatFilesHonorsIgnoreFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("Mkdir(.git): %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("Mkdir(docs): %v", err)
	}
	ignoredPath := filepath.Join(dir, "docs", "template.md")
	if err := os.WriteFile(filepath.Join(dir, ".prettierignore"), []byte("docs/\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.prettierignore): %v", err)
	}
	if err := os.WriteFile(ignoredPath, []byte("#  Hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(template.md): %v", err)
	}

	if err := formatFiles(afero.NewOsFs(), []string{ignoredPath}, byteformat.FormatMarkdown, false, true); err != nil {
		t.Fatalf("formatFiles() error = %v", err)
	}
	if got, err := os.ReadFile(ignoredPath); err != nil {
		t.Fatalf("ReadFile(template.md): %v", err)
	} else if want := "#  Hello\n"; string(got) != want {
		t.Errorf("ignored file = %q, want %q", got, want)
	}
}

func TestFormatFilesForceBypassesIgnoreFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("Mkdir(.git): %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("Mkdir(docs): %v", err)
	}
	ignoredPath := filepath.Join(dir, "docs", "template.md")
	if err := os.WriteFile(filepath.Join(dir, ".scutignore"), []byte("docs/\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.scutignore): %v", err)
	}
	if err := os.WriteFile(ignoredPath, []byte("#  Hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(template.md): %v", err)
	}
	if err := os.Chmod(ignoredPath, 0o640); err != nil {
		t.Fatalf("Chmod(template.md): %v", err)
	}

	if err := formatFiles(afero.NewOsFs(), []string{ignoredPath}, byteformat.FormatMarkdown, true, false); err != nil {
		t.Fatalf("formatFiles() error = %v", err)
	}
	if got, err := os.ReadFile(ignoredPath); err != nil {
		t.Fatalf("ReadFile(template.md): %v", err)
	} else if want := "# Hello\n"; string(got) != want {
		t.Errorf("formatted file = %q, want %q", got, want)
	}
	if info, err := os.Stat(ignoredPath); err != nil {
		t.Fatalf("Stat(template.md): %v", err)
	} else if got, want := info.Mode().Perm(), os.FileMode(0o640); got != want {
		t.Errorf("formatted file mode = %v, want %v", got, want)
	}
}

func TestFormatStdinPreservesFrontMatter(t *testing.T) {
	input := "+++\ntitle = \"Thing\"\n+++\n#  Hello\n"
	want := "+++\ntitle = \"Thing\"\n+++\n# Hello\n"

	var stdout bytes.Buffer
	if err := formatStdin(&stdout, bytes.NewBufferString(input), byteformat.FormatMarkdown); err != nil {
		t.Fatalf("formatStdin() error: %v", err)
	}
	if got := stdout.String(); got != want {
		t.Errorf("formatStdin() output = %q, want %q", got, want)
	}
}

func TestFormatStdinPassesThroughDeclinedInput(t *testing.T) {
	input := "package"

	var stdout bytes.Buffer
	if err := formatStdin(&stdout, bytes.NewBufferString(input), byteformat.FormatGo); err != nil {
		t.Fatalf("formatStdin() error: %v", err)
	}
	if got := stdout.String(); got != input {
		t.Errorf("formatStdin() output = %q, want %q", got, input)
	}
}

func TestFormatFilesCheckReportsChangesWithoutWriting(t *testing.T) {
	fs := afero.NewMemMapFs()
	files := []string{"/second.md", "/first.md", "/second.md"}
	for _, path := range files {
		if err := afero.WriteFile(fs, path, []byte("#  Hello\n"), 0o640); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
	}

	err := formatFiles(fs, files, byteformat.FormatMarkdown, false, true)
	checkErr, ok := errors.AsType[*checkError](err)
	if !ok {
		t.Fatalf("formatFiles() error = %v, want *checkError", err)
	}
	if got, want := checkErr.Error(), "format check failed; files would change: /second.md, /first.md"; got != want {
		t.Errorf("check error = %q, want %q", got, want)
	}
	for _, path := range files {
		got, readErr := afero.ReadFile(fs, path)
		if readErr != nil {
			t.Fatalf("ReadFile(%s): %v", path, readErr)
		}
		if want := "#  Hello\n"; string(got) != want {
			t.Errorf("checked file %s = %q, want %q", path, got, want)
		}
	}
}

func TestFormatFilesCheckPassesFormattedFiles(t *testing.T) {
	fs := afero.NewMemMapFs()
	if err := afero.WriteFile(fs, "/formatted.md", []byte("# Hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(formatted.md): %v", err)
	}
	if err := formatFiles(fs, []string{"/formatted.md"}, byteformat.FormatMarkdown, false, true); err != nil {
		t.Fatalf("formatFiles() error = %v", err)
	}
}

func TestFormatFilesStopsAfterPartialFailure(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.md")
	missing := filepath.Join(dir, "missing.md")
	if err := os.WriteFile(first, []byte("#  Hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(first.md): %v", err)
	}

	err := formatFiles(afero.NewOsFs(), []string{first, missing}, byteformat.FormatMarkdown, false, false)
	if err == nil || !strings.Contains(err.Error(), "checking ignores for "+missing) {
		t.Fatalf("formatFiles() error = %v", err)
	}
	if got, readErr := os.ReadFile(first); readErr != nil {
		t.Fatalf("ReadFile(first.md): %v", readErr)
	} else if want := "# Hello\n"; string(got) != want {
		t.Errorf("first file = %q, want %q", got, want)
	}
}

func TestFormatFilesPreservesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic-link creation requires additional privileges on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.md")
	link := filepath.Join(dir, "link.md")
	if err := os.WriteFile(target, []byte("#  Hello\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(target.md): %v", err)
	}
	if err := os.Symlink(filepath.Base(target), link); err != nil {
		t.Fatalf("Symlink(): %v", err)
	}

	if err := formatFiles(afero.NewOsFs(), []string{link}, byteformat.FormatMarkdown, false, false); err != nil {
		t.Fatalf("formatFiles() error = %v", err)
	}
	if info, err := os.Lstat(link); err != nil {
		t.Fatalf("Lstat(link.md): %v", err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("link.md mode = %v, want symbolic link", info.Mode())
	}
	if got, err := os.ReadFile(target); err != nil {
		t.Fatalf("ReadFile(target.md): %v", err)
	} else if want := "# Hello\n"; string(got) != want {
		t.Errorf("target file = %q, want %q", got, want)
	}
	if info, err := os.Stat(target); err != nil {
		t.Fatalf("Stat(target.md): %v", err)
	} else if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Errorf("target file mode = %v, want %v", got, want)
	}
}

type renameFailFS struct {
	afero.Fs
}

func (renameFailFS) Rename(_, _ string) error {
	return errors.New("injected rename failure")
}

func TestWriteFileAtomicLeavesOriginalOnRenameFailure(t *testing.T) {
	fs := renameFailFS{Fs: afero.NewMemMapFs()}
	const path = "/document.md"
	if err := afero.WriteFile(fs, path, []byte("original\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(document.md): %v", err)
	}

	err := writeFileAtomic(fs, path, []byte("replacement\n"), 0o640)
	if err == nil || !strings.Contains(err.Error(), "injected rename failure") {
		t.Fatalf("writeFileAtomic() error = %v", err)
	}
	if got, readErr := afero.ReadFile(fs, path); readErr != nil {
		t.Fatalf("ReadFile(document.md): %v", readErr)
	} else if want := "original\n"; string(got) != want {
		t.Errorf("original file = %q, want %q", got, want)
	}
	entries, err := afero.ReadDir(fs, "/")
	if err != nil {
		t.Fatalf("ReadDir(/): %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "document.md" {
		t.Errorf("directory entries = %v, want only document.md", entries)
	}
}

func TestMarkdownCheckRequiresFileArguments(t *testing.T) {
	cmd := markdownCmd{Check: true}
	err := cmd.Run(strings.NewReader("#  Hello\n"), &bytes.Buffer{}, afero.NewMemMapFs())
	if err == nil || err.Error() != "--check requires at least one file" {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestMarkdownRunFormatsNamedFileWithoutStdout(t *testing.T) {
	fs := afero.NewMemMapFs()
	const path = "/document.md"
	if err := afero.WriteFile(fs, path, []byte("#  Hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(document.md): %v", err)
	}
	cmd := markdownCmd{Files: []string{path}}
	var stdout bytes.Buffer
	if err := cmd.Run(strings.NewReader("ignored"), &stdout, fs); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("Run() stdout = %q, want empty", stdout.String())
	}
	if got, err := afero.ReadFile(fs, path); err != nil {
		t.Fatalf("ReadFile(document.md): %v", err)
	} else if want := "# Hello\n"; string(got) != want {
		t.Errorf("document.md = %q, want %q", got, want)
	}
}

func TestMarkdownFormatConfig(t *testing.T) {
	printWidth := 100
	tabWidth := 4
	cmd := markdownCmd{
		ProseWrap:   "never",
		PrintWidth:  &printWidth,
		TabWidth:    &tabWidth,
		SingleQuote: true,
	}

	got, err := cmd.formatConfig()
	if err != nil {
		t.Fatalf("formatConfig() error = %v", err)
	}
	if got.ProseWrap != byteformat.MarkdownProseWrapNever || got.PrintWidth != 100 || got.TabWidth != 4 || !got.SingleQuote {
		t.Errorf("formatConfig() = %+v", got)
	}
}

func TestMarkdownFormatConfigUsesDefaults(t *testing.T) {
	got, err := (&markdownCmd{}).formatConfig()
	if err != nil {
		t.Fatalf("formatConfig() error = %v", err)
	}
	want := byteformat.DefaultMarkdownConfig()
	if got != want {
		t.Errorf("formatConfig() = %+v, want %+v", got, want)
	}
}

func TestMarkdownRunAppliesOptions(t *testing.T) {
	cmd := markdownCmd{ProseWrap: "never", SingleQuote: true}
	stdin := strings.NewReader("This is a line\nthat continues.\n\n[link](https://example.com \"Title\")\n")
	var stdout bytes.Buffer

	if err := cmd.Run(stdin, &stdout, afero.NewMemMapFs()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := "This is a line that continues.\n\n[link](https://example.com 'Title')\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestMarkdownOptionValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "unknown prose wrap",
			args: []string{"document.md", "--prose-wrap=sometimes"},
			want: `--prose-wrap must be one of "preserve","always","never" but got "sometimes"`,
		},
		{
			name: "zero print width",
			args: []string{"document.md", "--print-width=0"},
			want: "--print-width must be greater than zero",
		},
		{
			name: "negative tab width",
			args: []string{"document.md", "--tab-width=-1"},
			want: "--tab-width must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := kong.Must(&markdownCmd{})
			_, err := parser.Parse(tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Parse(%q) error = %v, want containing %q", tt.args, err, tt.want)
			}
		})
	}
}
