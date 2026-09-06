package godoc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
	modzip "golang.org/x/mod/zip"
)

const (
	verificationGoSum       = "go.sum"
	verificationGoCache     = "go-cache"
	verificationSumDBPrefix = "sumdb:"
	verificationPolicyOff   = "policy:gosumdb-off"
	verificationPolicySkip  = "policy:gonosumdb"
)

// ArchiveVerification binds a canonical module content hash to the authority
// that accepted it.
type ArchiveVerification struct {
	Hash   string
	Source string
}

// ArchiveVerifier validates a complete module archive before it is consumed
// or published.
type ArchiveVerifier interface {
	Verify(context.Context, ModuleArchive) (ArchiveVerification, error)
}

// ChecksumDatabase returns authenticated go.sum lines for a module version.
type ChecksumDatabase interface {
	Lookup(context.Context, module.Version) (string, []string, error)
}

// ModuleArchiveVerifier applies Go's archive checksum policy without editing
// go.mod, go.sum, or the Go module cache.
type ModuleArchiveVerifier struct {
	Policy ModuleDownloadPolicy
	GoSums GoSumFiles
	SumDB  ChecksumDatabase
}

func (v ModuleArchiveVerifier) Verify(ctx context.Context, archive ModuleArchive) (ArchiveVerification, error) {
	if err := ctx.Err(); err != nil {
		return ArchiveVerification{}, err
	}
	hash, err := hashModuleArchive(archive)
	if err != nil {
		return ArchiveVerification{}, fmt.Errorf("verifying module archive %s@%s: %w", archive.Module.Path, archive.Module.Version, err)
	}

	matched, err := v.GoSums.Match(archive.Module, hash)
	if err != nil {
		return ArchiveVerification{}, err
	}
	if matched {
		return ArchiveVerification{Hash: hash, Source: verificationGoSum}, nil
	}
	if archive.Verification.Hash != "" {
		if err := checkArchiveVerification(archive.Verification); err != nil {
			return ArchiveVerification{}, err
		}
		if archive.Verification.Hash != hash {
			return ArchiveVerification{}, fmt.Errorf("module archive verification hash mismatch for %s@%s: got %s, want %s", archive.Module.Path, archive.Module.Version, hash, archive.Verification.Hash)
		}
	}
	if v.Policy.GOSUMDB == "off" {
		if archive.Verification.Hash != "" {
			return archive.Verification, nil
		}
		return ArchiveVerification{Hash: hash, Source: verificationPolicyOff}, nil
	}
	if matchesModulePattern(v.Policy.GONOSUMDB, archive.Module.Path) {
		if archive.Verification.Hash != "" {
			return archive.Verification, nil
		}
		return ArchiveVerification{Hash: hash, Source: verificationPolicySkip}, nil
	}
	if archive.Verification.Hash != "" && archiveVerificationSatisfiesPolicy(archive.Verification, v.Policy) {
		return archive.Verification, nil
	}
	if v.SumDB == nil {
		return ArchiveVerification{}, fmt.Errorf("verifying %s@%s: checksum database is unavailable", archive.Module.Path, archive.Module.Version)
	}

	database, lines, err := v.SumDB.Lookup(ctx, archive.Module)
	if err != nil {
		return ArchiveVerification{}, fmt.Errorf("verifying %s@%s: %w", archive.Module.Path, archive.Module.Version, err)
	}
	wantPrefix := archive.Module.Path + " " + archive.Module.Version + " h1:"
	want := archive.Module.Path + " " + archive.Module.Version + " " + hash
	for _, line := range lines {
		if line == want {
			return ArchiveVerification{Hash: hash, Source: verificationSumDBPrefix + database}, nil
		}
		if strings.HasPrefix(line, wantPrefix) {
			return ArchiveVerification{}, checksumMismatch(archive.Module, hash, database, strings.TrimPrefix(line, archive.Module.Path+" "+archive.Module.Version+" "))
		}
	}
	return ArchiveVerification{}, fmt.Errorf("verifying %s@%s: checksum missing from %s response", archive.Module.Path, archive.Module.Version, database)
}

func archiveVerificationSatisfiesPolicy(verification ArchiveVerification, policy ModuleDownloadPolicy) bool {
	switch verification.Source {
	case verificationGoSum, verificationGoCache:
		return true
	case verificationPolicyOff, verificationPolicySkip:
		return false
	}
	if !strings.HasPrefix(verification.Source, verificationSumDBPrefix) {
		return false
	}
	name, _, _, _, err := parseChecksumDatabase(policy.GOSUMDB)
	return err == nil && strings.TrimPrefix(verification.Source, verificationSumDBPrefix) == name
}

func hashModuleArchive(archive ModuleArchive) (string, error) {
	if err := checkModuleVersion(archive.Module); err != nil {
		return "", err
	}
	if len(archive.Data) == 0 {
		return "", errors.New("module archive is empty")
	}
	file, err := os.CreateTemp("", ".scut-module-*.zip")
	if err != nil {
		return "", err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err := file.Write(archive.Data); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if _, err := modzip.CheckZip(archive.Module, name); err != nil {
		return "", err
	}
	return dirhash.HashZip(name, dirhash.DefaultHash)
}

func checksumMismatch(mod module.Version, downloaded, source, recorded string) error {
	return fmt.Errorf("verifying %s@%s: checksum mismatch: downloaded %s; %s has %s", mod.Path, mod.Version, downloaded, source, recorded)
}

// GoSumFiles is the set of checksum files applicable to the active module or
// workspace. Missing files are equivalent to empty files.
type GoSumFiles struct {
	FS    afero.Fs
	Paths []string
}

// Match reports whether an h1 checksum matches. A conflicting h1 checksum is
// always an error, even if another applicable file contains a match.
func (s GoSumFiles) Match(mod module.Version, hash string) (bool, error) {
	fsys := s.FS
	if fsys == nil {
		fsys = afero.NewOsFs()
	}
	matched := false
	for _, name := range s.Paths {
		file, err := fsys.Open(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("reading checksum file %s: %w", name, err)
		}
		scanner := bufio.NewScanner(file)
		line := 0
		for scanner.Scan() {
			line++
			fields := strings.Fields(scanner.Text())
			if len(fields) == 0 {
				continue
			}
			if len(fields) != 3 {
				file.Close()
				return false, fmt.Errorf("malformed checksum file %s:%d: wrong number of fields %d", name, line, len(fields))
			}
			if fields[0] != mod.Path || fields[1] != mod.Version || !strings.HasPrefix(fields[2], "h1:") {
				continue
			}
			if fields[2] != hash {
				file.Close()
				return false, checksumMismatch(mod, hash, name, fields[2])
			}
			matched = true
		}
		scanErr := scanner.Err()
		closeErr := file.Close()
		if scanErr != nil {
			return false, fmt.Errorf("reading checksum file %s: %w", name, scanErr)
		}
		if closeErr != nil {
			return false, fmt.Errorf("closing checksum file %s: %w", name, closeErr)
		}
	}
	return matched, nil
}

func applicableGoSumFiles(fs afero.Fs, workDir, moduleDir string) []string {
	if fs == nil {
		fs = afero.NewOsFs()
	}
	workFile := configuredGoWork(fs, workDir)
	if workFile == "" {
		if moduleDir == "" {
			return nil
		}
		return []string{filepath.Join(moduleDir, "go.sum")}
	}

	paths := []string{workFile + ".sum"}
	data, err := afero.ReadFile(fs, workFile)
	if err != nil {
		return paths
	}
	work, err := modfile.ParseWork(workFile, data, nil)
	if err != nil {
		return paths
	}
	root := filepath.Dir(workFile)
	for _, use := range work.Use {
		dir := use.Path
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, filepath.FromSlash(dir))
		}
		paths = append(paths, filepath.Join(filepath.Clean(dir), "go.sum"))
	}
	return dedupePaths(paths)
}

func configuredGoWork(fs afero.Fs, start string) string {
	value := os.Getenv("GOWORK")
	if value == "off" {
		return ""
	}
	if value != "" && value != "auto" {
		if filepath.IsAbs(value) {
			return filepath.Clean(value)
		}
		return ""
	}
	for dir := filepath.Clean(start); ; dir = filepath.Dir(dir) {
		name := filepath.Join(dir, "go.work")
		if _, err := fs.Stat(name); err == nil {
			return name
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
	}
}

func dedupePaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	result := make([]string, 0, len(paths))
	for _, name := range paths {
		name = filepath.Clean(name)
		if seen[name] {
			continue
		}
		seen[name] = true
		result = append(result, name)
	}
	return result
}

var _ ArchiveVerifier = ModuleArchiveVerifier{}
