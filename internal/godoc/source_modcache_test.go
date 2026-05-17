package godoc

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"golang.org/x/mod/module"
)

func TestModCacheFetcherLoadsDependencyByLongestPrefix(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeModuleCacheFile(t, fs, "/mod", "github.com/foo/bar", "v1.0.0", "root.go", "package bar\n")
	writeModuleCacheFile(t, fs, "/mod", "github.com/foo/bar/sub", "v2.0.0", "deep/deep.go", "package deep\n")

	fetcher := ModCacheFetcher{
		FS:       fs,
		CacheDir: "/mod",
		Deps: map[string]module.Version{
			"github.com/foo/bar":     {Path: "github.com/foo/bar", Version: "v1.0.0"},
			"github.com/foo/bar/sub": {Path: "github.com/foo/bar/sub", Version: "v2.0.0"},
		},
	}

	source, err := fetcher.Fetch(context.Background(), "github.com/foo/bar/sub/deep", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if got, want := source.ImportPath, "github.com/foo/bar/sub/deep"; got != want {
		t.Fatalf("ImportPath = %q, want %q", got, want)
	}
	if got, want := source.Dir, filepath.Join("/mod", "github.com/foo/bar/sub@v2.0.0", "deep"); got != want {
		t.Fatalf("Dir = %q, want %q", got, want)
	}
	if got, want := len(source.Files), 1; got != want {
		t.Fatalf("len(Files) = %d, want %d", got, want)
	}
}

func TestModCacheFetcherProbesHighestCachedVersion(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeModuleCacheFile(t, fs, "/mod", "github.com/other/lib", "v1.0.0", "lib.go", "package lib\n")
	writeModuleCacheFile(t, fs, "/mod", "github.com/other/lib", "v1.2.0", "lib.go", "package lib\n")

	fetcher := ModCacheFetcher{FS: fs, CacheDir: "/mod"}
	source, err := fetcher.Fetch(context.Background(), "github.com/other/lib", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if got, want := source.Dir, filepath.Join("/mod", "github.com/other/lib@v1.2.0"); got != want {
		t.Fatalf("Dir = %q, want %q", got, want)
	}
}

func TestModCacheFetcherHandlesEscapedPaths(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeModuleCacheFile(t, fs, "/mod", "github.com/BurntSushi/toml", "v1.3.0", "toml.go", "package toml\n")

	fetcher := ModCacheFetcher{
		FS:       fs,
		CacheDir: "/mod",
		Deps: map[string]module.Version{
			"github.com/BurntSushi/toml": {Path: "github.com/BurntSushi/toml", Version: "v1.3.0"},
		},
	}
	source, err := fetcher.Fetch(context.Background(), "github.com/BurntSushi/toml", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if got, want := len(source.Files), 1; got != want {
		t.Fatalf("len(Files) = %d, want %d", got, want)
	}
}

func TestModCacheFetcherExplicitVersionMismatchMisses(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeModuleCacheFile(t, fs, "/mod", "github.com/foo/bar", "v1.0.0", "bar.go", "package bar\n")

	fetcher := ModCacheFetcher{
		FS:       fs,
		CacheDir: "/mod",
		Deps: map[string]module.Version{
			"github.com/foo/bar": {Path: "github.com/foo/bar", Version: "v1.0.0"},
		},
	}

	_, err := fetcher.Fetch(context.Background(), "github.com/foo/bar", Options{Version: "v2.0.0"})
	if !errors.Is(err, ErrSourceNotApplicable) {
		t.Fatalf("Fetch() error = %v, want ErrSourceNotApplicable", err)
	}
}

func TestModCacheFetcherDisabledWhenCacheDirEmpty(t *testing.T) {
	fetcher := ModCacheFetcher{FS: afero.NewMemMapFs()}

	_, err := fetcher.Fetch(context.Background(), "github.com/foo/bar", Options{})
	if !errors.Is(err, ErrSourceNotApplicable) {
		t.Fatalf("Fetch() error = %v, want ErrSourceNotApplicable", err)
	}
}

func TestWriteCacheRoundTrip(t *testing.T) {
	fs := afero.NewMemMapFs()
	resolved := ResolvedModule{
		Path:        "github.com/cached/mod",
		Version:     "v1.5.0",
		PackagePath: "github.com/cached/mod/sub",
	}
	files := []SourceFile{{Name: "sub.go", Data: []byte("package sub\n")}}

	if err := WriteCache(fs, "/mod", resolved, files); err != nil {
		t.Fatalf("WriteCache() error = %v", err)
	}

	fetcher := ModCacheFetcher{FS: fs, CacheDir: "/mod"}
	source, err := fetcher.Fetch(context.Background(), "github.com/cached/mod/sub", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got, want := len(source.Files), 1; got != want {
		t.Fatalf("len(Files) = %d, want %d", got, want)
	}
	if got, want := string(source.Files[0].Data), "package sub\n"; got != want {
		t.Fatalf("Data = %q, want %q", got, want)
	}
}

func writeModuleCacheFile(t *testing.T, fs afero.Fs, cacheDir, modPath, version, name, data string) {
	t.Helper()
	dir := moduleCacheDir(t, cacheDir, modPath, version)
	if err := fs.MkdirAll(filepath.Dir(filepath.Join(dir, filepath.FromSlash(name))), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := afero.WriteFile(fs, filepath.Join(dir, filepath.FromSlash(name)), []byte(data), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func moduleCacheDir(t *testing.T, cacheDir, modPath, version string) string {
	t.Helper()
	escaped, err := module.EscapePath(modPath)
	if err != nil {
		t.Fatalf("EscapePath() error = %v", err)
	}
	escapedVersion, err := module.EscapeVersion(version)
	if err != nil {
		t.Fatalf("EscapeVersion() error = %v", err)
	}
	return filepath.Join(cacheDir, filepath.FromSlash(escaped)+"@"+escapedVersion)
}
