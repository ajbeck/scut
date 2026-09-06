package godoc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golang.org/x/mod/sumdb/dirhash"
	modzip "golang.org/x/mod/zip"
)

// GoCacheArchiveReader reads complete module archives created by the Go tool.
// It never creates or repairs cache artifacts.
type GoCacheArchiveReader struct {
	Root string
}

func (r GoCacheArchiveReader) Get(ctx context.Context, mod module.Version) (ModuleArchive, error) {
	if err := ctx.Err(); err != nil {
		return ModuleArchive{}, err
	}
	zipPath, err := r.cachePath(mod, "zip")
	if err != nil {
		return ModuleArchive{}, err
	}
	hashPath, err := r.cachePath(mod, "ziphash")
	if err != nil {
		return ModuleArchive{}, err
	}
	wantHash, err := os.ReadFile(hashPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ModuleArchive{}, archiveNotFound(mod)
		}
		return ModuleArchive{}, fmt.Errorf("reading Go module ziphash for %s@%s: %w", mod.Path, mod.Version, err)
	}
	if _, err := modzip.CheckZip(mod, zipPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ModuleArchive{}, archiveNotFound(mod)
		}
		return ModuleArchive{}, fmt.Errorf("validating Go module archive %s@%s: %w", mod.Path, mod.Version, err)
	}
	gotHash, err := dirhash.HashZip(zipPath, dirhash.DefaultHash)
	if err != nil {
		return ModuleArchive{}, fmt.Errorf("hashing Go module archive %s@%s: %w", mod.Path, mod.Version, err)
	}
	if want := strings.TrimSpace(string(wantHash)); want == "" || gotHash != want {
		return ModuleArchive{}, fmt.Errorf("Go module archive hash mismatch for %s@%s: got %s, want %s", mod.Path, mod.Version, gotHash, want)
	}
	data, err := os.ReadFile(zipPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ModuleArchive{}, archiveNotFound(mod)
		}
		return ModuleArchive{}, fmt.Errorf("reading Go module archive %s@%s: %w", mod.Path, mod.Version, err)
	}
	if err := ctx.Err(); err != nil {
		return ModuleArchive{}, err
	}
	return ModuleArchive{
		Module:       mod,
		Data:         data,
		Verification: ArchiveVerification{Hash: gotHash, Source: verificationGoCache},
	}, nil
}

func (r GoCacheArchiveReader) Versions(ctx context.Context, modulePath string) ([]module.Version, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	versionDir, err := r.versionDir(modulePath)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(versionDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading Go module version cache for %s: %w", modulePath, err)
	}
	var versions []module.Version
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".zip") {
			continue
		}
		escapedVersion := strings.TrimSuffix(entry.Name(), ".zip")
		version, err := module.UnescapeVersion(escapedVersion)
		if err != nil {
			continue
		}
		mod := module.Version{Path: modulePath, Version: version}
		if checkModuleVersion(mod) != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(versionDir, escapedVersion+".ziphash")); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("reading Go module ziphash for %s@%s: %w", modulePath, version, err)
		}
		versions = append(versions, mod)
	}
	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(versions[i].Version, versions[j].Version) > 0
	})
	return versions, nil
}

func (r GoCacheArchiveReader) cachePath(mod module.Version, suffix string) (string, error) {
	if err := checkModuleVersion(mod); err != nil {
		return "", err
	}
	versionDir, err := r.versionDir(mod.Path)
	if err != nil {
		return "", err
	}
	escapedVersion, err := module.EscapeVersion(mod.Version)
	if err != nil {
		return "", err
	}
	return filepath.Join(versionDir, escapedVersion+"."+suffix), nil
}

func (r GoCacheArchiveReader) versionDir(modulePath string) (string, error) {
	if r.Root == "" {
		return "", errors.New("Go module cache root is empty")
	}
	if err := module.CheckPath(modulePath); err != nil {
		return "", err
	}
	escapedPath, err := module.EscapePath(modulePath)
	if err != nil {
		return "", err
	}
	return filepath.Join(r.Root, "cache", "download", filepath.FromSlash(escapedPath), "@v"), nil
}

// GoCacheFetcher loads package source from verified Go download-cache archives.
type GoCacheFetcher struct {
	Reader   GoCacheArchiveReader
	Selector ModuleSelector
	Verifier ArchiveVerifier
}

func (f GoCacheFetcher) Fetch(ctx context.Context, pkg string, opts Options) (PackageSource, error) {
	if f.Reader.Root == "" {
		return PackageSource{}, ErrSourceNotApplicable
	}
	if f.Selector != nil {
		if selected, ok := f.Selector.Select(ctx, pkg, opts); ok && selected.Dir == "" && selected.Source.Version != "" {
			return f.fetchSelected(ctx, pkg, selected)
		}
	}

	var cachedAbsent *cachedPackageAbsentError
	for _, modulePath := range modulePathCandidates(pkg) {
		mods, err := f.candidates(ctx, modulePath, opts.Version)
		if err != nil {
			return PackageSource{}, err
		}
		for _, mod := range mods {
			archive, err := f.Reader.Get(ctx, mod)
			if errors.Is(err, ErrArchiveNotFound) {
				continue
			}
			if err != nil {
				return PackageSource{}, err
			}
			archive, err = f.verifyArchive(ctx, archive)
			if err != nil {
				return PackageSource{}, err
			}
			source, err := packageSourceFromArchive(archive, pkg, "go-cache")
			if errors.Is(err, ErrNoGoFiles) {
				cachedAbsent = &cachedPackageAbsentError{Module: mod, Package: pkg}
				break
			}
			if err != nil {
				return PackageSource{}, err
			}
			return source, nil
		}
	}
	if cachedAbsent != nil {
		return PackageSource{}, cachedAbsent
	}
	return PackageSource{}, ErrSourceNotApplicable
}

func (f GoCacheFetcher) fetchSelected(ctx context.Context, pkg string, selected ModuleSelection) (PackageSource, error) {
	archive, err := f.Reader.Get(ctx, selected.Source)
	if errors.Is(err, ErrArchiveNotFound) {
		return PackageSource{}, ErrSourceNotApplicable
	}
	if err != nil {
		return PackageSource{}, err
	}
	archive, err = f.verifyArchive(ctx, archive)
	if err != nil {
		return PackageSource{}, err
	}
	source, err := packageSourceFromSelectedArchive(archive, pkg, selected.Module, "go-cache")
	if errors.Is(err, ErrNoGoFiles) {
		return PackageSource{}, &cachedPackageAbsentError{Module: selected.Module, Package: pkg}
	}
	return source, err
}

func (f GoCacheFetcher) verifyArchive(ctx context.Context, archive ModuleArchive) (ModuleArchive, error) {
	if f.Verifier == nil {
		return archive, nil
	}
	verification, err := f.Verifier.Verify(ctx, archive)
	if err != nil {
		return ModuleArchive{}, err
	}
	archive.Verification = verification
	return archive, nil
}

func (f GoCacheFetcher) candidates(ctx context.Context, modulePath, version string) ([]module.Version, error) {
	if version != "" && version != "latest" {
		mod := module.Version{Path: modulePath, Version: version}
		if checkModuleVersion(mod) != nil {
			return nil, nil
		}
		return []module.Version{mod}, nil
	}
	versions, err := f.Reader.Versions(ctx, modulePath)
	if err != nil || len(versions) == 0 {
		return versions, err
	}
	return versions[:1], nil
}

var _ SourceFetcher = GoCacheFetcher{}
