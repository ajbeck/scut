package godoc

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/mod/module"
)

// ArchiveFetcher loads package source from the scut-owned module archive
// cache without consulting the network.
type ArchiveFetcher struct {
	Store ArchiveStore
}

func (f ArchiveFetcher) Fetch(ctx context.Context, pkg string, opts Options) (PackageSource, error) {
	if f.Store == nil {
		return PackageSource{}, ErrSourceNotApplicable
	}
	for _, modulePath := range modulePathCandidates(pkg) {
		mod, err := f.cachedVersion(ctx, modulePath, opts.Version)
		if errors.Is(err, ErrArchiveNotFound) {
			continue
		}
		if err != nil {
			return PackageSource{}, err
		}
		archive, err := f.Store.Get(ctx, mod)
		if errors.Is(err, ErrArchiveNotFound) {
			continue
		}
		if err != nil {
			return PackageSource{}, err
		}
		source, err := packageSourceFromArchive(archive, pkg, "scut-cache")
		if errors.Is(err, ErrNoGoFiles) {
			continue
		}
		if err != nil {
			return PackageSource{}, err
		}
		return source, nil
	}
	return PackageSource{}, ErrSourceNotApplicable
}

func (f ArchiveFetcher) cachedVersion(ctx context.Context, modulePath, version string) (module.Version, error) {
	if version == "" || version == "latest" {
		return f.Store.Latest(ctx, modulePath)
	}
	mod := module.Version{Path: modulePath, Version: version}
	if err := checkModuleVersion(mod); err != nil {
		return module.Version{}, fmtArchiveNotFound(modulePath, version)
	}
	return mod, nil
}

func packageSourceFromArchive(archive ModuleArchive, pkg, source string) (PackageSource, error) {
	files, err := extractPackageZip(archive.Module.Path, archive.Module.Version, pkg, archive.Data)
	if err != nil {
		return PackageSource{}, err
	}
	return PackageSource{
		ImportPath: pkg,
		Dir:        filepath.Join("/", source, filepath.FromSlash(pkg)),
		Files:      files,
		Module:     archive.Module,
		Version:    archive.Module.Version,
	}, nil
}

func modulePathCandidates(pkg string) []string {
	parts := strings.Split(pkg, "/")
	minParts := 2
	if len(parts) >= 3 && isCommonGitHost(parts[0]) {
		minParts = 3
	}
	var candidates []string
	for i := len(parts); i >= minParts; i-- {
		candidates = append(candidates, strings.Join(parts[:i], "/"))
	}
	return candidates
}

func fmtArchiveNotFound(modulePath, version string) error {
	return fmt.Errorf("%w: %s@%s", ErrArchiveNotFound, modulePath, version)
}

var _ SourceFetcher = ArchiveFetcher{}
