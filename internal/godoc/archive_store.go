package godoc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
	modzip "golang.org/x/mod/zip"
)

var ErrArchiveNotFound = errors.New("module archive not found")

const (
	archiveFileName  = "module.zip"
	archiveHashName  = "module.ziphash"
	verificationName = "verification"
	revisionFileName = "revision"
	latestFileName   = "latest"
)

// ModuleArchive is a complete canonical module ZIP and its immutable source
// identity, when the source backend exposes one.
type ModuleArchive struct {
	Module       module.Version
	Data         []byte
	Revision     string
	Verification ArchiveVerification
}

// ArchiveStore persists complete immutable module archives.
type ArchiveStore interface {
	Get(context.Context, module.Version) (ModuleArchive, error)
	Put(context.Context, ModuleArchive) error
	Latest(context.Context, string) (module.Version, error)
	SetLatest(context.Context, module.Version) error
}

// FileArchiveStore stores each module version as one atomically published
// directory beneath Root.
type FileArchiveStore struct {
	Root string
}

func (s FileArchiveStore) Get(ctx context.Context, mod module.Version) (ModuleArchive, error) {
	if err := ctx.Err(); err != nil {
		return ModuleArchive{}, err
	}
	entryDir, err := s.entryDir(mod)
	if err != nil {
		return ModuleArchive{}, err
	}
	zipPath := filepath.Join(entryDir, archiveFileName)
	if _, err := modzip.CheckZip(mod, zipPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ModuleArchive{}, archiveNotFound(mod)
		}
		return ModuleArchive{}, fmt.Errorf("validating cached module archive %s@%s: %w", mod.Path, mod.Version, err)
	}
	if err := verifyArchiveHash(zipPath, filepath.Join(entryDir, archiveHashName)); err != nil {
		return ModuleArchive{}, fmt.Errorf("validating cached module archive hash %s@%s: %w", mod.Path, mod.Version, err)
	}
	data, err := os.ReadFile(zipPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ModuleArchive{}, archiveNotFound(mod)
		}
		return ModuleArchive{}, fmt.Errorf("reading cached module archive %s@%s: %w", mod.Path, mod.Version, err)
	}
	revision, err := readOptionalRevision(filepath.Join(entryDir, revisionFileName))
	if err != nil {
		return ModuleArchive{}, fmt.Errorf("reading cached module revision %s@%s: %w", mod.Path, mod.Version, err)
	}
	if err := ctx.Err(); err != nil {
		return ModuleArchive{}, err
	}
	verification, err := readArchiveVerification(filepath.Join(entryDir, verificationName))
	if err != nil {
		return ModuleArchive{}, fmt.Errorf("reading cached module verification %s@%s: %w", mod.Path, mod.Version, err)
	}
	gotHash, hashErr := dirhash.HashZip(zipPath, dirhash.DefaultHash)
	if hashErr != nil {
		return ModuleArchive{}, fmt.Errorf("hashing cached module archive %s@%s: %w", mod.Path, mod.Version, hashErr)
	}
	if gotHash != verification.Hash {
		return ModuleArchive{}, fmt.Errorf("cached module verification hash mismatch for %s@%s: got %s, want %s", mod.Path, mod.Version, gotHash, verification.Hash)
	}
	return ModuleArchive{Module: mod, Data: data, Revision: revision, Verification: verification}, nil
}

func (s FileArchiveStore) Put(ctx context.Context, archive ModuleArchive) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	entryDir, err := s.entryDir(archive.Module)
	if err != nil {
		return err
	}
	if len(archive.Data) == 0 {
		return errors.New("module archive is empty")
	}
	if err := checkRevision(archive.Revision); err != nil {
		return err
	}
	if err := checkArchiveVerification(archive.Verification); err != nil {
		return err
	}
	if _, err := s.Get(ctx, archive.Module); err == nil {
		return nil
	} else if !errors.Is(err, ErrArchiveNotFound) {
		return err
	}

	parent := filepath.Dir(entryDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("creating module archive directory: %w", err)
	}
	tempDir, err := os.MkdirTemp(parent, ".archive-*")
	if err != nil {
		return fmt.Errorf("creating temporary module archive directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	zipPath := filepath.Join(tempDir, archiveFileName)
	if err := writeFileSync(zipPath, archive.Data, 0o644); err != nil {
		return fmt.Errorf("staging module archive: %w", err)
	}
	if _, err := modzip.CheckZip(archive.Module, zipPath); err != nil {
		return fmt.Errorf("validating module archive %s@%s: %w", archive.Module.Path, archive.Module.Version, err)
	}
	hash, err := dirhash.HashZip(zipPath, dirhash.DefaultHash)
	if err != nil {
		return fmt.Errorf("hashing module archive %s@%s: %w", archive.Module.Path, archive.Module.Version, err)
	}
	if err := writeFileSync(filepath.Join(tempDir, archiveHashName), []byte(hash+"\n"), 0o644); err != nil {
		return fmt.Errorf("staging module archive hash: %w", err)
	}
	if archive.Verification.Hash != hash {
		return fmt.Errorf("module archive verification hash mismatch for %s@%s: got %s, want %s", archive.Module.Path, archive.Module.Version, hash, archive.Verification.Hash)
	}
	if err := writeFileSync(filepath.Join(tempDir, verificationName), formatArchiveVerification(archive.Verification), 0o644); err != nil {
		return fmt.Errorf("staging module archive verification: %w", err)
	}
	if archive.Revision != "" {
		if err := writeFileSync(filepath.Join(tempDir, revisionFileName), []byte(archive.Revision), 0o644); err != nil {
			return fmt.Errorf("staging module revision: %w", err)
		}
	}
	if err := os.Chmod(tempDir, 0o755); err != nil {
		return fmt.Errorf("setting module archive directory permissions: %w", err)
	}
	if err := syncDirectory(tempDir); err != nil {
		return fmt.Errorf("syncing module archive directory: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(tempDir, entryDir); err != nil {
		if _, existingErr := s.Get(ctx, archive.Module); existingErr == nil {
			return nil
		}
		return fmt.Errorf("publishing module archive %s@%s: %w", archive.Module.Path, archive.Module.Version, err)
	}
	if err := syncDirectory(parent); err != nil {
		return fmt.Errorf("syncing module archive parent directory: %w", err)
	}
	return nil
}

func (s FileArchiveStore) Latest(ctx context.Context, modulePath string) (module.Version, error) {
	if err := ctx.Err(); err != nil {
		return module.Version{}, err
	}
	moduleDir, err := s.moduleDir(modulePath)
	if err != nil {
		return module.Version{}, err
	}
	data, err := os.ReadFile(filepath.Join(moduleDir, "@v", latestFileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return module.Version{}, fmt.Errorf("%w: latest version for %s", ErrArchiveNotFound, modulePath)
		}
		return module.Version{}, fmt.Errorf("reading latest cached module version for %s: %w", modulePath, err)
	}
	mod := module.Version{Path: modulePath, Version: strings.TrimSpace(string(data))}
	if err := checkModuleVersion(mod); err != nil {
		return module.Version{}, fmt.Errorf("invalid latest cached module version for %s: %w", modulePath, err)
	}
	return mod, nil
}

func (s FileArchiveStore) SetLatest(ctx context.Context, mod module.Version) error {
	if _, err := s.Get(ctx, mod); err != nil {
		return err
	}
	moduleDir, err := s.moduleDir(mod.Path)
	if err != nil {
		return err
	}
	versionDir := filepath.Join(moduleDir, "@v")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		return fmt.Errorf("creating latest module version directory: %w", err)
	}
	temp, err := os.CreateTemp(versionDir, ".latest-*")
	if err != nil {
		return fmt.Errorf("creating temporary latest module version: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.WriteString(mod.Version + "\n"); err != nil {
		temp.Close()
		return fmt.Errorf("writing latest module version: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("syncing latest module version: %w", err)
	}
	if err := temp.Chmod(0o644); err != nil {
		temp.Close()
		return fmt.Errorf("setting latest module version permissions: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("closing latest module version: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(tempName, filepath.Join(versionDir, latestFileName)); err != nil {
		return fmt.Errorf("publishing latest module version: %w", err)
	}
	return syncDirectory(versionDir)
}

// DefaultModuleArchiveCacheDir returns the scut-owned module archive cache.
func DefaultModuleArchiveCacheDir() (string, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolving user cache directory: %w", err)
	}
	return filepath.Join(root, "scut", "gotools", "modules"), nil
}

func (s FileArchiveStore) entryDir(mod module.Version) (string, error) {
	if err := checkModuleVersion(mod); err != nil {
		return "", err
	}
	moduleDir, err := s.moduleDir(mod.Path)
	if err != nil {
		return "", err
	}
	escapedVersion, err := module.EscapeVersion(mod.Version)
	if err != nil {
		return "", err
	}
	return filepath.Join(moduleDir, "@v", escapedVersion), nil
}

func (s FileArchiveStore) moduleDir(modulePath string) (string, error) {
	if s.Root == "" {
		return "", errors.New("module archive cache root is empty")
	}
	if err := module.CheckPath(modulePath); err != nil {
		return "", err
	}
	escapedPath, err := module.EscapePath(modulePath)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.Root, filepath.FromSlash(escapedPath)), nil
}

func checkModuleVersion(mod module.Version) error {
	if canonical := module.CanonicalVersion(mod.Version); canonical != mod.Version {
		return fmt.Errorf("module version %q is not canonical", mod.Version)
	}
	return module.Check(mod.Path, mod.Version)
}

func checkRevision(revision string) error {
	if revision != strings.TrimSpace(revision) || strings.ContainsAny(revision, "\r\n") {
		return errors.New("module archive revision must be a single trimmed line")
	}
	return nil
}

func archiveNotFound(mod module.Version) error {
	return fmt.Errorf("%w: %s@%s", ErrArchiveNotFound, mod.Path, mod.Version)
}

func readOptionalRevision(name string) (string, error) {
	data, err := os.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	revision := string(data)
	if err := checkRevision(revision); err != nil {
		return "", err
	}
	return revision, nil
}

func checkArchiveVerification(verification ArchiveVerification) error {
	if !strings.HasPrefix(verification.Hash, "h1:") || strings.TrimSpace(verification.Hash) != verification.Hash || strings.ContainsAny(verification.Hash, "\r\n") {
		return errors.New("module archive verification requires a valid h1 hash")
	}
	if verification.Source == "" || strings.TrimSpace(verification.Source) != verification.Source || strings.ContainsAny(verification.Source, "\r\n \t") {
		return errors.New("module archive verification requires a single-token source")
	}
	return nil
}

func formatArchiveVerification(verification ArchiveVerification) []byte {
	return []byte("v1\n" + verification.Hash + "\n" + verification.Source + "\n")
}

func readArchiveVerification(name string) (ArchiveVerification, error) {
	data, err := os.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return ArchiveVerification{}, errors.New("module archive verification is missing")
	}
	if err != nil {
		return ArchiveVerification{}, err
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 3 || lines[0] != "v1" {
		return ArchiveVerification{}, errors.New("malformed module archive verification")
	}
	verification := ArchiveVerification{Hash: lines[1], Source: lines[2]}
	if err := checkArchiveVerification(verification); err != nil {
		return ArchiveVerification{}, err
	}
	return verification, nil
}

func verifyArchiveHash(zipPath, hashPath string) error {
	want, err := os.ReadFile(hashPath)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("module archive hash is missing")
	}
	if err != nil {
		return err
	}
	wantHash := strings.TrimSpace(string(want))
	if wantHash == "" {
		return errors.New("empty archive hash")
	}
	gotHash, err := dirhash.HashZip(zipPath, dirhash.DefaultHash)
	if err != nil {
		return err
	}
	if gotHash != wantHash {
		return fmt.Errorf("archive hash mismatch: got %s, want %s", gotHash, wantHash)
	}
	return nil
}

func writeFileSync(name string, data []byte, perm os.FileMode) error {
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func syncDirectory(name string) error {
	dir, err := os.Open(name)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
