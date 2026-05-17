package godoc

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// ModCacheFetcher reads package source from an existing module cache.
type ModCacheFetcher struct {
	FS       afero.Fs
	CacheDir string
	Deps     map[string]module.Version
}

func (f ModCacheFetcher) Fetch(_ context.Context, pkg string, opts Options) (PackageSource, error) {
	if f.CacheDir == "" {
		return PackageSource{}, ErrSourceNotApplicable
	}

	mod, ok := f.resolveFromDeps(pkg)
	if !ok {
		mod, ok = f.resolveByProbing(pkg)
	}
	if !ok {
		return PackageSource{}, ErrSourceNotApplicable
	}

	if opts.Version != "" && opts.Version != "latest" && opts.Version != mod.Version {
		return PackageSource{}, ErrSourceNotApplicable
	}

	dir, err := resolvedModulePackageDir(f.CacheDir, ResolvedModule{
		Path:        mod.Path,
		Version:     mod.Version,
		PackagePath: pkg,
	})
	if err != nil {
		return PackageSource{}, ErrSourceNotApplicable
	}
	files, err := readGoFiles(f.FS, dir)
	if errors.Is(err, ErrNoGoFiles) {
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

func (f ModCacheFetcher) resolveFromDeps(pkg string) (module.Version, bool) {
	var best module.Version
	for modPath, mod := range f.Deps {
		if pkg != modPath && !strings.HasPrefix(pkg, modPath+"/") {
			continue
		}
		if len(modPath) > len(best.Path) {
			best = mod
		}
	}
	return best, best.Path != ""
}

func (f ModCacheFetcher) resolveByProbing(pkg string) (module.Version, bool) {
	parts := strings.Split(pkg, "/")
	minParts := 2
	if len(parts) >= 3 && isCommonGitHost(parts[0]) {
		minParts = 3
	}

	for i := len(parts); i >= minParts; i-- {
		modPath := strings.Join(parts[:i], "/")
		version, ok := f.highestCachedVersion(modPath)
		if ok {
			return module.Version{Path: modPath, Version: version}, true
		}
	}
	return module.Version{}, false
}

func (f ModCacheFetcher) highestCachedVersion(modPath string) (string, bool) {
	escaped, err := module.EscapePath(modPath)
	if err != nil {
		return "", false
	}
	parent := filepath.Join(f.CacheDir, filepath.Dir(filepath.FromSlash(escaped)))
	base := filepath.Base(filepath.FromSlash(escaped)) + "@"

	entries, err := afero.ReadDir(f.FS, parent)
	if err != nil {
		return "", false
	}

	var best string
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), base) {
			continue
		}
		version := strings.TrimPrefix(entry.Name(), base)
		if best == "" || semver.Compare(version, best) > 0 {
			best = version
		}
	}
	return best, best != ""
}

func isCommonGitHost(host string) bool {
	return host == "github.com" || host == "gitlab.com" || host == "bitbucket.org"
}
