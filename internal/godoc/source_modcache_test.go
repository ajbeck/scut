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
	if got, want := source.Module, (module.Version{Path: "github.com/foo/bar/sub", Version: "v2.0.0"}); got != want {
		t.Fatalf("Module = %#v, want %#v", got, want)
	}
	if got, want := source.Version, "v2.0.0"; got != want {
		t.Fatalf("Version = %q, want %q", got, want)
	}
}

func TestModCacheFetcherReportsExistingPackageWithoutGoFiles(t *testing.T) {
	fs := afero.NewMemMapFs()
	mod := module.Version{Path: "github.com/foo/bar", Version: "v1.0.0"}
	writeTestFile(t, fs, filepath.Join(moduleCacheDir(t, "/mod", mod.Path, mod.Version), "README.md"), []byte("docs\n"))

	fetcher := ModCacheFetcher{
		FS:       fs,
		CacheDir: "/mod",
		Deps:     map[string]module.Version{mod.Path: mod},
	}

	_, err := fetcher.Fetch(context.Background(), mod.Path, Options{Version: mod.Version})
	absent, ok := errors.AsType[*cachedPackageAbsentError](err)
	if !ok {
		t.Fatalf("Fetch() error = %v, want cachedPackageAbsentError", err)
	}
	if got, want := absent.Module, mod; got != want {
		t.Fatalf("absent.Module = %#v, want %#v", got, want)
	}
	if got, want := absent.Package, mod.Path; got != want {
		t.Fatalf("absent.Package = %q, want %q", got, want)
	}
}

func TestModCacheFetcherTreatsMissingPackageDirectoryAsCacheMiss(t *testing.T) {
	fs := afero.NewMemMapFs()
	mod := module.Version{Path: "github.com/foo/bar", Version: "v1.0.0"}
	writeModuleCacheFile(t, fs, "/mod", mod.Path, mod.Version, "other/other.go", "package other\n")

	fetcher := ModCacheFetcher{
		FS:       fs,
		CacheDir: "/mod",
		Deps:     map[string]module.Version{mod.Path: mod},
	}

	_, err := fetcher.Fetch(context.Background(), mod.Path+"/missing", Options{Version: mod.Version})
	if !errors.Is(err, ErrSourceNotApplicable) {
		t.Fatalf("Fetch() error = %v, want ErrSourceNotApplicable", err)
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
