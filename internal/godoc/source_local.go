package godoc

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/afero"
)

var (
	ErrSourceNotApplicable = errors.New("source fetcher not applicable")
	ErrNoGoFiles           = errors.New("no Go source files")
	ErrOutsideModule       = errors.New("local package path is outside module")
)

// LocalSourceFetcher loads packages addressed by ./ or ../ paths.
type LocalSourceFetcher struct {
	FS         afero.Fs
	ModuleDir  string
	ModulePath string
}

func (f LocalSourceFetcher) Fetch(_ context.Context, pkg string, _ Options) (PackageSource, error) {
	if !strings.HasPrefix(pkg, "./") && !strings.HasPrefix(pkg, "../") && pkg != "." && pkg != ".." {
		return PackageSource{}, ErrSourceNotApplicable
	}

	moduleDir := filepath.Clean(f.ModuleDir)
	dir := filepath.Clean(filepath.Join(moduleDir, pkg))
	rel, err := filepath.Rel(moduleDir, dir)
	if err != nil {
		return PackageSource{}, fmt.Errorf("resolving local package path: %w", err)
	}
	if rel != "." && !filepath.IsLocal(rel) {
		return PackageSource{}, ErrOutsideModule
	}

	files, err := readGoFiles(f.FS, dir)
	if err != nil {
		return PackageSource{}, err
	}

	importPath := f.ModulePath
	if rel != "." {
		importPath = path.Join(f.ModulePath, filepath.ToSlash(rel))
	}
	return PackageSource{
		ImportPath: importPath,
		Dir:        dir,
		Files:      files,
	}, nil
}

// StdlibSourceFetcher loads packages from GOROOT/src.
type StdlibSourceFetcher struct {
	FS     afero.Fs
	GOROOT string
}

func (f StdlibSourceFetcher) Fetch(_ context.Context, pkg string, _ Options) (PackageSource, error) {
	if strings.HasPrefix(pkg, ".") {
		return PackageSource{}, ErrSourceNotApplicable
	}

	dir := filepath.Join(f.GOROOT, "src", filepath.FromSlash(pkg))
	files, err := readGoFiles(f.FS, dir)
	if errors.Is(err, ErrNoGoFiles) || errors.Is(err, afero.ErrFileNotFound) {
		return PackageSource{}, ErrSourceNotApplicable
	}
	if err != nil {
		return PackageSource{}, err
	}
	return PackageSource{
		ImportPath: pkg,
		Dir:        dir,
		Files:      files,
	}, nil
}

func readGoFiles(fs afero.Fs, dir string) ([]SourceFile, error) {
	entries, err := afero.ReadDir(fs, dir)
	if err != nil {
		if errors.Is(err, afero.ErrFileNotFound) {
			return nil, ErrNoGoFiles
		}
		return nil, fmt.Errorf("reading package directory %s: %w", dir, err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	files := make([]SourceFile, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		filename := filepath.Join(dir, name)
		data, err := afero.ReadFile(fs, filename)
		if err != nil {
			return nil, fmt.Errorf("reading source file %s: %w", filename, err)
		}
		files = append(files, SourceFile{Name: filename, Data: data})
	}
	if len(files) == 0 {
		return nil, ErrNoGoFiles
	}
	return files, nil
}
