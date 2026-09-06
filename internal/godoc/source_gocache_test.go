package godoc

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
)

func TestGoCacheFetcherUsesSelectedArchiveReadOnly(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	root := t.TempDir()
	wantData := "package pkg\n\nconst Source = \"archive\"\n"
	zipPath, hashPath := writeGoCacheArchive(t, root, mod, map[string]string{
		"pkg/pkg.go": wantData,
	}, "")
	writeGoCacheArchive(t, root, module.Version{Path: mod.Path, Version: "v1.9.0"}, map[string]string{
		"pkg/pkg.go": "package pkg\n\nconst Source = \"newer but not selected\"\n",
	}, "")
	zipData, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatalf("ReadFile(zip) error = %v", err)
	}
	hashData, err := os.ReadFile(hashPath)
	if err != nil {
		t.Fatalf("ReadFile(ziphash) error = %v", err)
	}
	oldTime := time.Unix(1_700_000_000, 0)
	for _, name := range []string{zipPath, hashPath} {
		if err := os.Chmod(name, 0o444); err != nil {
			t.Fatalf("Chmod(%s) error = %v", name, err)
		}
		if err := os.Chtimes(name, oldTime, oldTime); err != nil {
			t.Fatalf("Chtimes(%s) error = %v", name, err)
		}
	}
	extractedDir, err := moduleCachePath(root, mod.Path, mod.Version)
	if err != nil {
		t.Fatalf("moduleCachePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(extractedDir, "pkg"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(extractedDir, "pkg", "pkg.go"), []byte("package pkg\n\nconst Source = \"partial\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	fetcher := GoCacheFetcher{
		Reader:   GoCacheArchiveReader{Root: root},
		Selector: DependencySelector{mod.Path: mod},
	}
	source, err := fetcher.Fetch(t.Context(), mod.Path+"/pkg", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got, want := string(source.Files[0].Data), wantData; got != want {
		t.Fatalf("source data = %q, want %q", got, want)
	}
	for _, name := range []string{zipPath, hashPath} {
		info, err := os.Stat(name)
		if err != nil {
			t.Fatalf("Stat(%s) error = %v", name, err)
		}
		if got, want := info.Mode().Perm(), os.FileMode(0o444); got != want {
			t.Fatalf("mode for %s = %o, want %o", name, got, want)
		}
		if got := info.ModTime(); !got.Equal(oldTime) {
			t.Fatalf("modification time for %s = %s, want %s", name, got, oldTime)
		}
	}
	assertFileContents(t, zipPath, zipData)
	assertFileContents(t, hashPath, hashData)
}

func TestGoCacheFetcherRequiresZipHashAndIgnoresExtractedDirectory(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	root := t.TempDir()
	archive := moduleZip(t, mod.Path, mod.Version, map[string]string{"pkg/pkg.go": "package pkg\n"})
	reader := GoCacheArchiveReader{Root: root}
	zipPath, err := reader.cachePath(mod, "zip")
	if err != nil {
		t.Fatalf("cachePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(zipPath, archive, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	extractedDir, err := moduleCachePath(root, mod.Path, mod.Version)
	if err != nil {
		t.Fatalf("moduleCachePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(extractedDir, "pkg"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(extractedDir, "pkg", "pkg.go"), []byte("package pkg\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err = (GoCacheFetcher{Reader: reader}).Fetch(t.Context(), mod.Path+"/pkg", Options{Version: mod.Version})
	if !errors.Is(err, ErrSourceNotApplicable) {
		t.Fatalf("Fetch() error = %v, want ErrSourceNotApplicable", err)
	}
}

func TestGoCacheFetcherRejectsHashMismatch(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	root := t.TempDir()
	writeGoCacheArchive(t, root, mod, map[string]string{"pkg/pkg.go": "package pkg\n"}, "h1:wrong")

	_, err := (GoCacheFetcher{Reader: GoCacheArchiveReader{Root: root}}).Fetch(
		t.Context(),
		mod.Path+"/pkg",
		Options{Version: mod.Version},
	)
	if err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("Fetch() error = %v, want hash mismatch", err)
	}
}

func TestGoCacheFetcherUsesHighestCachedVersionWithoutSelection(t *testing.T) {
	root := t.TempDir()
	for _, version := range []string{"v1.0.0", "v1.4.0"} {
		mod := module.Version{Path: "example.com/acme/tool", Version: version}
		writeGoCacheArchive(t, root, mod, map[string]string{
			"tool.go": "package tool\n\nconst Version = \"" + version + "\"\n",
		}, "")
	}

	source, err := (GoCacheFetcher{Reader: GoCacheArchiveReader{Root: root}}).Fetch(
		t.Context(),
		"example.com/acme/tool",
		Options{},
	)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got, want := source.Module.Version, "v1.4.0"; got != want {
		t.Fatalf("Module.Version = %q, want %q", got, want)
	}
}

func TestGoCacheFetcherReadsVersionedReplacementArchive(t *testing.T) {
	logical := module.Version{Path: "example.com/lib", Version: "v1.2.3"}
	replacement := module.Version{Path: "example.com/fork", Version: "v1.4.0"}
	root := t.TempDir()
	writeGoCacheArchive(t, root, replacement, map[string]string{"pkg/pkg.go": "package pkg\n"}, "")
	fetcher := GoCacheFetcher{
		Reader: GoCacheArchiveReader{Root: root},
		Selector: fixedModuleSelector{selection: ModuleSelection{
			Module: logical,
			Source: replacement,
		}},
	}

	source, err := fetcher.Fetch(t.Context(), logical.Path+"/pkg", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if source.Module != logical {
		t.Fatalf("Module = %#v, want %#v", source.Module, logical)
	}
	if got, want := source.ImportPath, logical.Path+"/pkg"; got != want {
		t.Fatalf("ImportPath = %q, want %q", got, want)
	}
}

func writeGoCacheArchive(t *testing.T, root string, mod module.Version, files map[string]string, hash string) (string, string) {
	t.Helper()
	reader := GoCacheArchiveReader{Root: root}
	zipPath, err := reader.cachePath(mod, "zip")
	if err != nil {
		t.Fatalf("cachePath(zip) error = %v", err)
	}
	hashPath, err := reader.cachePath(mod, "ziphash")
	if err != nil {
		t.Fatalf("cachePath(ziphash) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	archive := moduleZip(t, mod.Path, mod.Version, files)
	if err := os.WriteFile(zipPath, archive, 0o644); err != nil {
		t.Fatalf("WriteFile(zip) error = %v", err)
	}
	if hash == "" {
		hash, err = dirhash.HashZip(zipPath, dirhash.DefaultHash)
		if err != nil {
			t.Fatalf("HashZip() error = %v", err)
		}
	}
	if err := os.WriteFile(hashPath, []byte(hash+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(ziphash) error = %v", err)
	}
	return zipPath, hashPath
}

type fixedModuleSelector struct {
	selection ModuleSelection
	ok        bool
}

func (s fixedModuleSelector) Select(context.Context, string, Options) (ModuleSelection, bool) {
	if !s.ok && s.selection.Module.Path == "" {
		return ModuleSelection{}, false
	}
	return s.selection, true
}

func assertFileContents(t *testing.T, name string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", name, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("contents of %s changed", name)
	}
}
