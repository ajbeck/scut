package godoc

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
)

func TestReadGoFilesExcludesTestsAndDirectories(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeTestFile(t, fs, "/repo/pkg/doc.go", []byte("package pkg\n"))
	writeTestFile(t, fs, "/repo/pkg/doc_test.go", []byte("package pkg\n"))
	writeTestFile(t, fs, "/repo/pkg/nested/ignore.go", []byte("package nested\n"))

	files, err := readGoFiles(fs, "/repo/pkg")
	if err != nil {
		t.Fatalf("readGoFiles() error = %v", err)
	}

	if got, want := len(files), 1; got != want {
		t.Fatalf("len(files) = %d, want %d", got, want)
	}
	if got, want := files[0].Name, filepath.Join("/repo/pkg", "doc.go"); got != want {
		t.Fatalf("files[0].Name = %q, want %q", got, want)
	}
}

func TestReadGoFilesExcludesIgnoredBuildFiles(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeTestFile(t, fs, "/repo/pkg/doc.go", []byte("package pkg\n"))
	writeTestFile(t, fs, "/repo/pkg/tool.go", []byte("//go:build ignore\n\npackage main\n"))

	files, err := readGoFiles(fs, "/repo/pkg")
	if err != nil {
		t.Fatalf("readGoFiles() error = %v", err)
	}

	if got, want := len(files), 1; got != want {
		t.Fatalf("len(files) = %d, want %d", got, want)
	}
	if got, want := files[0].Name, filepath.Join("/repo/pkg", "doc.go"); got != want {
		t.Fatalf("files[0].Name = %q, want %q", got, want)
	}
}

func TestReadGoFilesReturnsNoGoFiles(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeTestFile(t, fs, "/repo/pkg/README.md", []byte("docs\n"))

	_, err := readGoFiles(fs, "/repo/pkg")
	if !errors.Is(err, ErrNoGoFiles) {
		t.Fatalf("readGoFiles() error = %v, want ErrNoGoFiles", err)
	}
}

func TestLocalSourceFetcherLoadsRelativePackage(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeTestFile(t, fs, "/repo/internal/widget/widget.go", []byte("package widget\n"))

	fetcher := LocalSourceFetcher{
		FS:         fs,
		ModuleDir:  "/repo",
		ModulePath: "example.com/project",
	}
	source, err := fetcher.Fetch(context.Background(), "./internal/widget", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if got, want := source.ImportPath, "example.com/project/internal/widget"; got != want {
		t.Fatalf("ImportPath = %q, want %q", got, want)
	}
	if got, want := source.Dir, filepath.Join("/repo", "internal/widget"); got != want {
		t.Fatalf("Dir = %q, want %q", got, want)
	}
	if got, want := len(source.Files), 1; got != want {
		t.Fatalf("len(Files) = %d, want %d", got, want)
	}
}

func TestLocalSourceFetcherRejectsOutsideModule(t *testing.T) {
	fetcher := LocalSourceFetcher{
		FS:         afero.NewMemMapFs(),
		ModuleDir:  "/repo",
		ModulePath: "example.com/project",
	}

	_, err := fetcher.Fetch(context.Background(), "../outside", Options{})
	if !errors.Is(err, ErrOutsideModule) {
		t.Fatalf("Fetch() error = %v, want ErrOutsideModule", err)
	}
}

func TestLocalSourceFetcherSkipsImportPath(t *testing.T) {
	fetcher := LocalSourceFetcher{
		FS:         afero.NewMemMapFs(),
		ModuleDir:  "/repo",
		ModulePath: "example.com/project",
	}

	_, err := fetcher.Fetch(context.Background(), "encoding/json", Options{})
	if !errors.Is(err, ErrSourceNotApplicable) {
		t.Fatalf("Fetch() error = %v, want ErrSourceNotApplicable", err)
	}
}

func TestReplaceSourceFetcherLoadsPackageFromLocalReplacement(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeTestFile(t, fs, "/workspace/lib/sub/doc.go", []byte("package sub\n"))

	fetcher := ReplaceSourceFetcher{
		FS: fs,
		Replacements: map[string]string{
			"example.com/lib": "/workspace/lib",
		},
	}
	source, err := fetcher.Fetch(context.Background(), "example.com/lib/sub", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if got, want := source.ImportPath, "example.com/lib/sub"; got != want {
		t.Fatalf("ImportPath = %q, want %q", got, want)
	}
	if got, want := source.Dir, filepath.Join("/workspace/lib", "sub"); got != want {
		t.Fatalf("Dir = %q, want %q", got, want)
	}
	if got, want := len(source.Files), 1; got != want {
		t.Fatalf("len(Files) = %d, want %d", got, want)
	}
}

func TestReplaceSourceFetcherRejectsPackageOutsideReplacementRoot(t *testing.T) {
	fetcher := ReplaceSourceFetcher{
		FS: afero.NewMemMapFs(),
		Replacements: map[string]string{
			"example.com/lib": "/workspace/lib",
		},
	}

	_, err := fetcher.Fetch(context.Background(), "example.com/lib/../other", Options{})
	if !errors.Is(err, ErrOutsideModule) {
		t.Fatalf("Fetch() error = %v, want ErrOutsideModule", err)
	}
}

func TestStdlibSourceFetcherLoadsPackage(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeTestFile(t, fs, "/goroot/src/encoding/json/encode.go", []byte("package json\n"))

	fetcher := StdlibSourceFetcher{FS: fs, GOROOT: "/goroot"}
	source, err := fetcher.Fetch(context.Background(), "encoding/json", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if got, want := source.ImportPath, "encoding/json"; got != want {
		t.Fatalf("ImportPath = %q, want %q", got, want)
	}
	if got, want := source.Dir, filepath.Join("/goroot/src", "encoding/json"); got != want {
		t.Fatalf("Dir = %q, want %q", got, want)
	}
	if got, want := len(source.Files), 1; got != want {
		t.Fatalf("len(Files) = %d, want %d", got, want)
	}
}

func TestStdlibSourceFetcherReturnsNotApplicableWhenMissing(t *testing.T) {
	fetcher := StdlibSourceFetcher{FS: afero.NewMemMapFs(), GOROOT: "/goroot"}

	_, err := fetcher.Fetch(context.Background(), "github.com/example/mod", Options{})
	if !errors.Is(err, ErrSourceNotApplicable) {
		t.Fatalf("Fetch() error = %v, want ErrSourceNotApplicable", err)
	}
}

func writeTestFile(t *testing.T, fs afero.Fs, name string, data []byte) {
	t.Helper()
	if err := fs.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := afero.WriteFile(fs, name, data, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
