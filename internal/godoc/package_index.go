package godoc

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/mod/module"
)

// PackageIndex finds locally known packages by import path suffix.
type PackageIndex interface {
	MatchSuffix(string) ([]IndexedPackage, error)
}

// IndexedPackage is one locally known package match.
type IndexedPackage struct {
	ImportPath string
	Dir        string
}

// LocalPackageIndex indexes packages that are already known on local disk.
type LocalPackageIndex struct {
	FS         afero.Fs
	ModuleDir  string
	ModulePath string
	GOROOT     string
	ModCache   string
}

func (i LocalPackageIndex) MatchSuffix(suffix string) ([]IndexedPackage, error) {
	for _, index := range []func() ([]IndexedPackage, error){
		func() ([]IndexedPackage, error) { return indexStdlibPackages(i.fs(), i.GOROOT) },
		func() ([]IndexedPackage, error) { return indexCurrentModulePackages(i.fs(), i.ModuleDir, i.ModulePath) },
		func() ([]IndexedPackage, error) { return indexModuleCachePackages(i.fs(), i.ModCache) },
	} {
		pkgs, err := index()
		if err != nil {
			return nil, err
		}
		group := filterIndexedPackages(pkgs, suffix)
		if len(group) == 0 {
			continue
		}
		sortIndexedPackages(group)
		return dedupeIndexedPackages(group), nil
	}
	return nil, nil
}

func (i LocalPackageIndex) fs() afero.Fs {
	if i.FS != nil {
		return i.FS
	}
	return afero.NewOsFs()
}

func indexStdlibPackages(fs afero.Fs, goroot string) ([]IndexedPackage, error) {
	if goroot == "" {
		return nil, nil
	}
	root := filepath.Join(goroot, "src")
	return indexPackagesUnder(fs, root, func(dir string) (string, bool) {
		rel, err := filepath.Rel(root, dir)
		if err != nil || rel == "." || !filepath.IsLocal(rel) {
			return "", false
		}
		return filepath.ToSlash(rel), true
	})
}

func indexCurrentModulePackages(fs afero.Fs, moduleDir, modulePath string) ([]IndexedPackage, error) {
	if moduleDir == "" || modulePath == "" {
		return nil, nil
	}
	root := filepath.Clean(moduleDir)
	return indexPackagesUnder(fs, root, func(dir string) (string, bool) {
		rel, err := filepath.Rel(root, dir)
		if err != nil || !filepath.IsLocal(rel) {
			return "", false
		}
		if rel == "." {
			return modulePath, true
		}
		return modulePath + "/" + filepath.ToSlash(rel), true
	})
}

func indexModuleCachePackages(fs afero.Fs, cacheDir string) ([]IndexedPackage, error) {
	if cacheDir == "" {
		return nil, nil
	}
	var pkgs []IndexedPackage
	cacheDir = filepath.Clean(cacheDir)
	err := walkDirs(fs, cacheDir, func(dir string) (bool, error) {
		if shouldSkipPackageIndexDir(filepath.Base(dir)) {
			return false, nil
		}
		modPath, ok := modulePathFromCacheDir(cacheDir, dir)
		if !ok {
			return true, nil
		}
		moduleRoot := dir
		modulePkgs, err := indexPackagesUnder(fs, moduleRoot, func(packageDir string) (string, bool) {
			rel, err := filepath.Rel(moduleRoot, packageDir)
			if err != nil || !filepath.IsLocal(rel) {
				return "", false
			}
			if rel == "." {
				return modPath, true
			}
			return modPath + "/" + filepath.ToSlash(rel), true
		})
		if err != nil {
			return false, err
		}
		pkgs = append(pkgs, modulePkgs...)
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return dedupeIndexedPackages(pkgs), nil
}

func indexPackagesUnder(fs afero.Fs, root string, importPath func(string) (string, bool)) ([]IndexedPackage, error) {
	var pkgs []IndexedPackage
	err := walkDirs(fs, filepath.Clean(root), func(dir string) (bool, error) {
		if dir != filepath.Clean(root) && shouldSkipPackageIndexDir(filepath.Base(dir)) {
			return false, nil
		}
		_, err := readGoFiles(fs, dir)
		if err == nil {
			path, ok := importPath(dir)
			if ok {
				pkgs = append(pkgs, IndexedPackage{ImportPath: path, Dir: filepath.Clean(dir)})
			}
			return true, nil
		}
		if errors.Is(err, ErrNoGoFiles) || errors.Is(err, afero.ErrFileNotFound) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return nil, err
	}
	sortIndexedPackages(pkgs)
	return pkgs, nil
}

func walkDirs(fs afero.Fs, root string, visit func(string) (descend bool, err error)) error {
	entries, err := afero.ReadDir(fs, root)
	if err != nil {
		if errors.Is(err, afero.ErrFileNotFound) {
			return nil
		}
		return fmt.Errorf("reading package index directory %s: %w", root, err)
	}
	descend, err := visit(root)
	if err != nil || !descend {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := walkDirs(fs, filepath.Join(root, entry.Name()), visit); err != nil {
			return err
		}
	}
	return nil
}

func modulePathFromCacheDir(cacheDir, dir string) (string, bool) {
	rel, err := filepath.Rel(cacheDir, dir)
	if err != nil || !filepath.IsLocal(rel) {
		return "", false
	}
	slashRel := filepath.ToSlash(rel)
	at := strings.LastIndex(slashRel, "@")
	if at < 0 {
		return "", false
	}
	escapedPath := slashRel[:at]
	if escapedPath == "" || strings.Contains(slashRel[at+1:], "/") {
		return "", false
	}
	modPath, err := module.UnescapePath(escapedPath)
	if err != nil {
		return "", false
	}
	return modPath, true
}

func filterIndexedPackages(pkgs []IndexedPackage, suffix string) []IndexedPackage {
	var matches []IndexedPackage
	for _, pkg := range pkgs {
		if pkg.ImportPath == suffix || strings.HasSuffix(pkg.ImportPath, "/"+suffix) {
			matches = append(matches, pkg)
		}
	}
	return matches
}

func sortIndexedPackages(pkgs []IndexedPackage) {
	sort.Slice(pkgs, func(i, j int) bool {
		leftDepth := strings.Count(pkgs[i].ImportPath, "/")
		rightDepth := strings.Count(pkgs[j].ImportPath, "/")
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return pkgs[i].ImportPath < pkgs[j].ImportPath
	})
}

func dedupeIndexedPackages(pkgs []IndexedPackage) []IndexedPackage {
	seen := map[string]bool{}
	var deduped []IndexedPackage
	for _, pkg := range pkgs {
		if seen[pkg.ImportPath] {
			continue
		}
		seen[pkg.ImportPath] = true
		deduped = append(deduped, pkg)
	}
	return deduped
}

func shouldSkipPackageIndexDir(name string) bool {
	return name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".")
}

type emptyPackageIndex struct{}

func (emptyPackageIndex) MatchSuffix(string) ([]IndexedPackage, error) {
	return nil, nil
}
