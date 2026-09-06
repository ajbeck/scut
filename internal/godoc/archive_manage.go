package godoc

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/mod/module"
	modzip "golang.org/x/mod/zip"
)

// ModuleCacheEntryStatus describes whether a scut-owned archive entry is ready
// for use.
type ModuleCacheEntryStatus string

const (
	ModuleCacheEntryValid      ModuleCacheEntryStatus = "valid"
	ModuleCacheEntryIncomplete ModuleCacheEntryStatus = "incomplete"
	ModuleCacheEntryInvalid    ModuleCacheEntryStatus = "invalid"
)

// ModuleCacheEntry is one canonical version directory in the scut-owned cache.
type ModuleCacheEntry struct {
	Module   module.Version
	Path     string
	Size     int64
	Modified time.Time
	Revision string
	Latest   bool
	Status   ModuleCacheEntryStatus
	Problem  string
}

// ModuleCacheProblem describes cache content that cannot be associated with a
// canonical module version.
type ModuleCacheProblem struct {
	Path    string
	Problem string
}

// ModuleCacheInventory is a deterministic snapshot of the scut-owned cache.
type ModuleCacheInventory struct {
	Root     string
	Entries  []ModuleCacheEntry
	Problems []ModuleCacheProblem
	Size     int64
}

// Filter returns the portion of an inventory associated with one module or
// exact module version.
func (i ModuleCacheInventory) Filter(target ModuleCacheTarget) ModuleCacheInventory {
	filtered := ModuleCacheInventory{Root: i.Root}
	for _, entry := range i.Entries {
		if entry.Module.Path != target.Path || (target.Version != "" && entry.Module.Version != target.Version) {
			continue
		}
		filtered.Entries = append(filtered.Entries, entry)
		filtered.Size += entry.Size
	}
	escapedPath, err := module.EscapePath(target.Path)
	if err != nil {
		return filtered
	}
	problemRoot := filepath.Join(i.Root, filepath.FromSlash(escapedPath), "@v")
	if target.Version != "" {
		escapedVersion, err := module.EscapeVersion(target.Version)
		if err == nil {
			problemRoot = filepath.Join(problemRoot, escapedVersion)
		}
	}
	for _, problem := range i.Problems {
		rel, err := filepath.Rel(problemRoot, problem.Path)
		if err == nil && (rel == "." || filepath.IsLocal(rel)) {
			filtered.Problems = append(filtered.Problems, problem)
		}
	}
	return filtered
}

// ModuleCacheTarget identifies either one module or one exact module version.
type ModuleCacheTarget struct {
	Path    string
	Version string
}

// ParseModuleCacheTarget parses module/path or module/path@version.
func ParseModuleCacheTarget(raw string) (ModuleCacheTarget, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ModuleCacheTarget{}, errors.New("module cache target is empty")
	}
	target := ModuleCacheTarget{Path: raw}
	if at := strings.LastIndex(raw, "@"); at >= 0 {
		target.Path = raw[:at]
		target.Version = raw[at+1:]
		if err := checkModuleVersion(module.Version{Path: target.Path, Version: target.Version}); err != nil {
			return ModuleCacheTarget{}, fmt.Errorf("invalid module cache target %q: %w", raw, err)
		}
		return target, nil
	}
	if err := module.CheckPath(target.Path); err != nil {
		return ModuleCacheTarget{}, fmt.Errorf("invalid module cache target %q: %w", raw, err)
	}
	return target, nil
}

// ModuleCacheRemoval summarizes a cache mutation.
type ModuleCacheRemoval struct {
	Entries int
	Bytes   int64
}

// ModuleCacheSizeError reports that non-entry cache artifacts prevented a
// prune operation from reaching its requested maximum size.
type ModuleCacheSizeError struct {
	Size  int64
	Limit int64
}

func (e ModuleCacheSizeError) Error() string {
	return fmt.Sprintf("module cache remains %d bytes, above max-size %d; run cache verify or cache clean", e.Size, e.Limit)
}

// ModuleCachePrunePolicy selects entries by publication age and total size.
type ModuleCachePrunePolicy struct {
	OlderThan *time.Duration
	MaxSize   *int64
	Now       time.Time
}

// InspectCache returns a read-only snapshot of the scut-owned module cache.
func (s FileArchiveStore) InspectCache(ctx context.Context) (ModuleCacheInventory, error) {
	root, err := s.managedRoot()
	if err != nil {
		return ModuleCacheInventory{}, err
	}
	inventory := ModuleCacheInventory{Root: root}
	if err := ctx.Err(); err != nil {
		return inventory, err
	}
	if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
		return inventory, nil
	} else if err != nil {
		return inventory, fmt.Errorf("reading module cache root: %w", err)
	}
	inventory.Size, _, err = directoryMetrics(root)
	if err != nil {
		return ModuleCacheInventory{}, fmt.Errorf("measuring module cache: %w", err)
	}

	err = filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == root {
			if !entry.IsDir() {
				return errors.New("module cache root is not a directory")
			}
			return nil
		}
		if entry.IsDir() && entry.Name() == "@v" {
			modulePath, parseErr := s.modulePathForVersionDir(root, name)
			if parseErr != nil {
				inventory.Problems = append(inventory.Problems, ModuleCacheProblem{Path: name, Problem: parseErr.Error()})
				return filepath.SkipDir
			}
			entries, problems, inspectErr := s.inspectVersionDir(ctx, modulePath, name)
			if inspectErr != nil {
				return inspectErr
			}
			inventory.Entries = append(inventory.Entries, entries...)
			inventory.Problems = append(inventory.Problems, problems...)
			return filepath.SkipDir
		}
		if !entry.IsDir() {
			inventory.Problems = append(inventory.Problems, ModuleCacheProblem{
				Path:    name,
				Problem: "unexpected file outside a module version directory",
			})
		}
		return nil
	})
	if err != nil {
		return ModuleCacheInventory{}, fmt.Errorf("inspecting module cache: %w", err)
	}
	sort.Slice(inventory.Entries, func(i, j int) bool {
		left, right := inventory.Entries[i].Module, inventory.Entries[j].Module
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		return left.Version < right.Version
	})
	sort.Slice(inventory.Problems, func(i, j int) bool {
		return inventory.Problems[i].Path < inventory.Problems[j].Path
	})
	return inventory, nil
}

func (s FileArchiveStore) inspectVersionDir(ctx context.Context, modulePath, versionDir string) ([]ModuleCacheEntry, []ModuleCacheProblem, error) {
	dirEntries, err := os.ReadDir(versionDir)
	if err != nil {
		return nil, nil, err
	}
	latest, latestProblem := readLatestAlias(modulePath, filepath.Join(versionDir, latestFileName))
	var entries []ModuleCacheEntry
	var problems []ModuleCacheProblem
	if latestProblem != "" {
		problems = append(problems, ModuleCacheProblem{Path: filepath.Join(versionDir, latestFileName), Problem: latestProblem})
	}
	latestFound := latest.Version == ""
	aliasSize := int64(0)
	if latest.Version != "" {
		if info, err := os.Lstat(filepath.Join(versionDir, latestFileName)); err == nil {
			aliasSize = info.Size()
		}
	}
	for _, dirEntry := range dirEntries {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		name := dirEntry.Name()
		entryPath := filepath.Join(versionDir, name)
		if name == latestFileName && !dirEntry.IsDir() {
			continue
		}
		if !dirEntry.IsDir() {
			problems = append(problems, ModuleCacheProblem{Path: entryPath, Problem: "unexpected module cache artifact"})
			continue
		}
		if strings.HasPrefix(name, ".archive-") {
			problems = append(problems, ModuleCacheProblem{Path: entryPath, Problem: "incomplete temporary archive entry"})
			continue
		}
		version, unescapeErr := module.UnescapeVersion(name)
		mod := module.Version{Path: modulePath, Version: version}
		if unescapeErr != nil || checkModuleVersion(mod) != nil {
			problems = append(problems, ModuleCacheProblem{Path: entryPath, Problem: "invalid module version directory"})
			continue
		}
		entry, inspectErr := inspectModuleCacheEntry(mod, entryPath)
		if inspectErr != nil {
			return nil, nil, inspectErr
		}
		entry.Latest = latest == mod
		if entry.Latest {
			entry.Size += aliasSize
		}
		latestFound = latestFound || entry.Latest
		entries = append(entries, entry)
	}
	if !latestFound {
		problems = append(problems, ModuleCacheProblem{
			Path:    filepath.Join(versionDir, latestFileName),
			Problem: fmt.Sprintf("latest alias references missing version %s", latest.Version),
		})
	}
	return entries, problems, nil
}

func inspectModuleCacheEntry(mod module.Version, entryPath string) (ModuleCacheEntry, error) {
	size, modified, err := directoryMetrics(entryPath)
	if err != nil {
		return ModuleCacheEntry{}, err
	}
	entry := ModuleCacheEntry{
		Module:   mod,
		Path:     entryPath,
		Size:     size,
		Modified: modified,
		Status:   ModuleCacheEntryValid,
	}

	dirEntries, err := os.ReadDir(entryPath)
	if err != nil {
		return ModuleCacheEntry{}, err
	}
	known := map[string]bool{archiveFileName: true, archiveHashName: true, revisionFileName: true}
	for _, child := range dirEntries {
		if !known[child.Name()] || child.IsDir() || child.Type()&os.ModeSymlink != 0 {
			entry.Status = ModuleCacheEntryInvalid
			entry.Problem = "unexpected content in module archive entry"
			return entry, nil
		}
	}

	zipPath := filepath.Join(entryPath, archiveFileName)
	if _, err := os.Lstat(zipPath); errors.Is(err, os.ErrNotExist) {
		entry.Status = ModuleCacheEntryIncomplete
		entry.Problem = "module archive is missing"
		return entry, nil
	} else if err != nil {
		return ModuleCacheEntry{}, err
	}
	if _, err := modzip.CheckZip(mod, zipPath); err != nil {
		entry.Status = ModuleCacheEntryInvalid
		entry.Problem = err.Error()
		return entry, nil
	}
	hashPath := filepath.Join(entryPath, archiveHashName)
	if _, err := os.Lstat(hashPath); errors.Is(err, os.ErrNotExist) {
		entry.Status = ModuleCacheEntryIncomplete
		entry.Problem = "module archive hash is missing"
		return entry, nil
	} else if err != nil {
		return ModuleCacheEntry{}, err
	}
	if err := verifyOptionalArchiveHash(zipPath, hashPath); err != nil {
		entry.Status = ModuleCacheEntryInvalid
		entry.Problem = err.Error()
		return entry, nil
	}
	revision, err := readOptionalRevision(filepath.Join(entryPath, revisionFileName))
	if err != nil {
		entry.Status = ModuleCacheEntryInvalid
		entry.Problem = err.Error()
		return entry, nil
	}
	entry.Revision = revision
	return entry, nil
}

func readLatestAlias(modulePath, name string) (module.Version, string) {
	info, err := os.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return module.Version{}, ""
	}
	if err != nil {
		return module.Version{}, err.Error()
	}
	if !info.Mode().IsRegular() {
		return module.Version{}, "latest alias is not a regular file"
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return module.Version{}, err.Error()
	}
	mod := module.Version{Path: modulePath, Version: strings.TrimSpace(string(data))}
	if err := checkModuleVersion(mod); err != nil {
		return module.Version{}, err.Error()
	}
	return mod, ""
}

func (s FileArchiveStore) modulePathForVersionDir(root, versionDir string) (string, error) {
	rel, err := filepath.Rel(root, filepath.Dir(versionDir))
	if err != nil || !filepath.IsLocal(rel) {
		return "", errors.New("module version directory escapes cache root")
	}
	escapedPath := filepath.ToSlash(rel)
	modulePath, err := module.UnescapePath(escapedPath)
	if err != nil {
		return "", fmt.Errorf("invalid escaped module path %q: %w", escapedPath, err)
	}
	if err := module.CheckPath(modulePath); err != nil {
		return "", err
	}
	return modulePath, nil
}

// RemoveCache removes one exact version or every version of one module.
func (s FileArchiveStore) RemoveCache(ctx context.Context, target ModuleCacheTarget) (ModuleCacheRemoval, error) {
	if err := ctx.Err(); err != nil {
		return ModuleCacheRemoval{}, err
	}
	if _, err := s.managedRoot(); err != nil {
		return ModuleCacheRemoval{}, err
	}
	if target.Version == "" {
		if err := module.CheckPath(target.Path); err != nil {
			return ModuleCacheRemoval{}, err
		}
		moduleDir, err := s.moduleDir(target.Path)
		if err != nil {
			return ModuleCacheRemoval{}, err
		}
		versionDir := filepath.Join(moduleDir, "@v")
		if err := s.validateRemovalPath(versionDir); err != nil {
			return ModuleCacheRemoval{}, err
		}
		removal, err := cacheRemovalMetrics(versionDir)
		if errors.Is(err, os.ErrNotExist) {
			return ModuleCacheRemoval{}, cacheTargetNotFound(target)
		}
		if err != nil {
			return ModuleCacheRemoval{}, err
		}
		if err := os.RemoveAll(versionDir); err != nil {
			return ModuleCacheRemoval{}, fmt.Errorf("removing module cache target %s: %w", target.Path, err)
		}
		removeDirIfEmpty(moduleDir)
		return removal, nil
	}

	mod := module.Version{Path: target.Path, Version: target.Version}
	entryDir, err := s.entryDir(mod)
	if err != nil {
		return ModuleCacheRemoval{}, err
	}
	if err := s.validateRemovalPath(entryDir); err != nil {
		return ModuleCacheRemoval{}, err
	}
	removal, err := cacheRemovalMetrics(entryDir)
	if errors.Is(err, os.ErrNotExist) {
		return ModuleCacheRemoval{}, cacheTargetNotFound(target)
	}
	if err != nil {
		return ModuleCacheRemoval{}, err
	}
	removal.Entries = 1
	versionDir := filepath.Dir(entryDir)
	if err := s.validateRemovalPath(filepath.Join(versionDir, latestFileName)); err != nil {
		return ModuleCacheRemoval{}, err
	}
	aliasBytes, err := matchingLatestAliasSize(versionDir, mod)
	if err != nil {
		return ModuleCacheRemoval{}, err
	}
	if err := os.RemoveAll(entryDir); err != nil {
		return ModuleCacheRemoval{}, fmt.Errorf("removing module cache target %s@%s: %w", target.Path, target.Version, err)
	}
	if err := removeMatchingLatestAlias(versionDir, mod); err != nil {
		return ModuleCacheRemoval{}, err
	}
	removal.Bytes += aliasBytes
	removeDirIfEmpty(versionDir)
	removeDirIfEmpty(filepath.Dir(versionDir))
	return removal, nil
}

// CleanCache removes the complete scut-owned module cache.
func (s FileArchiveStore) CleanCache(ctx context.Context) (ModuleCacheRemoval, error) {
	root, err := s.managedRoot()
	if err != nil {
		return ModuleCacheRemoval{}, err
	}
	if err := ctx.Err(); err != nil {
		return ModuleCacheRemoval{}, err
	}
	if err := s.validateRemovalPath(root); err != nil {
		return ModuleCacheRemoval{}, err
	}
	removal, err := cacheRemovalMetrics(root)
	if errors.Is(err, os.ErrNotExist) {
		return ModuleCacheRemoval{}, nil
	}
	if err != nil {
		return ModuleCacheRemoval{}, err
	}
	removal.Entries = 0
	if entries, err := countCacheEntries(root); err == nil {
		removal.Entries = entries
	}
	if err := os.RemoveAll(root); err != nil {
		return ModuleCacheRemoval{}, fmt.Errorf("cleaning module cache: %w", err)
	}
	return removal, nil
}

// PruneCache applies explicit age and size policies to canonical entries.
func (s FileArchiveStore) PruneCache(ctx context.Context, policy ModuleCachePrunePolicy) (ModuleCacheRemoval, error) {
	if policy.OlderThan == nil && policy.MaxSize == nil {
		return ModuleCacheRemoval{}, errors.New("module cache prune requires older-than or max-size")
	}
	if policy.OlderThan != nil && *policy.OlderThan <= 0 {
		return ModuleCacheRemoval{}, errors.New("older-than must be greater than zero")
	}
	if policy.MaxSize != nil && *policy.MaxSize < 0 {
		return ModuleCacheRemoval{}, errors.New("max-size must not be negative")
	}
	inventory, err := s.InspectCache(ctx)
	if err != nil {
		return ModuleCacheRemoval{}, err
	}
	now := policy.Now
	if now.IsZero() {
		now = time.Now()
	}
	entries := append([]ModuleCacheEntry(nil), inventory.Entries...)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Modified.Equal(entries[j].Modified) {
			if entries[i].Module.Path != entries[j].Module.Path {
				return entries[i].Module.Path < entries[j].Module.Path
			}
			return entries[i].Module.Version < entries[j].Module.Version
		}
		return entries[i].Modified.Before(entries[j].Modified)
	})

	selected := make(map[module.Version]bool)
	remainingSize := inventory.Size
	if policy.OlderThan != nil {
		cutoff := now.Add(-*policy.OlderThan)
		for _, entry := range entries {
			if entry.Modified.After(cutoff) {
				continue
			}
			selected[entry.Module] = true
			remainingSize -= entry.Size
		}
	}
	if policy.MaxSize != nil {
		for _, entry := range entries {
			if remainingSize <= *policy.MaxSize {
				break
			}
			if selected[entry.Module] {
				continue
			}
			selected[entry.Module] = true
			remainingSize -= entry.Size
		}
	}

	var total ModuleCacheRemoval
	for _, entry := range entries {
		if !selected[entry.Module] {
			continue
		}
		removed, err := s.RemoveCache(ctx, ModuleCacheTarget{Path: entry.Module.Path, Version: entry.Module.Version})
		if errors.Is(err, ErrArchiveNotFound) {
			continue
		}
		if err != nil {
			return total, err
		}
		total.Entries += removed.Entries
		total.Bytes += removed.Bytes
	}
	if policy.MaxSize != nil {
		remaining, err := s.InspectCache(ctx)
		if err != nil {
			return total, err
		}
		if remaining.Size > *policy.MaxSize {
			return total, ModuleCacheSizeError{Size: remaining.Size, Limit: *policy.MaxSize}
		}
	}
	return total, nil
}

func (s FileArchiveStore) managedRoot() (string, error) {
	if s.Root == "" {
		return "", errors.New("module archive cache root is empty")
	}
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return "", err
	}
	root = filepath.Clean(root)
	volumeRoot := filepath.VolumeName(root) + string(filepath.Separator)
	if root == volumeRoot {
		return "", errors.New("module archive cache root cannot be a filesystem root")
	}
	return root, nil
}

func (s FileArchiveStore) validateRemovalPath(target string) error {
	root, err := s.managedRoot()
	if err != nil {
		return err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	target = filepath.Clean(target)
	rel, err := filepath.Rel(root, target)
	if err != nil || (rel != "." && !filepath.IsLocal(rel)) {
		return errors.New("module cache removal target escapes owned cache root")
	}
	current := root
	parts := []string{}
	if rel != "." {
		parts = strings.Split(rel, string(filepath.Separator))
	}
	for index := -1; index < len(parts); index++ {
		if index >= 0 {
			current = filepath.Join(current, parts[index])
		}
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("checking module cache removal path %s: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("module cache removal path contains symlink: %s", current)
		}
	}
	return nil
}

func directoryMetrics(root string) (int64, time.Time, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return 0, time.Time{}, err
	}
	modified := info.ModTime()
	var size int64
	err = filepath.WalkDir(root, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			size += info.Size()
		}
		if info.ModTime().After(modified) {
			modified = info.ModTime()
		}
		return nil
	})
	return size, modified, err
}

func cacheRemovalMetrics(root string) (ModuleCacheRemoval, error) {
	size, _, err := directoryMetrics(root)
	if err != nil {
		return ModuleCacheRemoval{}, err
	}
	entries := 0
	dirEntries, readErr := os.ReadDir(root)
	if readErr == nil {
		for _, entry := range dirEntries {
			if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
				entries++
			}
		}
	}
	if entries == 0 {
		entries = 1
	}
	return ModuleCacheRemoval{Entries: entries, Bytes: size}, nil
}

func countCacheEntries(root string) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() || entry.Name() != "@v" {
			return nil
		}
		entries, err := os.ReadDir(name)
		if err != nil {
			return err
		}
		for _, candidate := range entries {
			if candidate.IsDir() && !strings.HasPrefix(candidate.Name(), ".") {
				count++
			}
		}
		return filepath.SkipDir
	})
	return count, err
}

func removeMatchingLatestAlias(versionDir string, mod module.Version) error {
	aliasPath := filepath.Join(versionDir, latestFileName)
	data, err := os.ReadFile(aliasPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading latest module cache alias: %w", err)
	}
	if strings.TrimSpace(string(data)) != mod.Version {
		return nil
	}
	if err := os.Remove(aliasPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing latest module cache alias: %w", err)
	}
	return nil
}

func matchingLatestAliasSize(versionDir string, mod module.Version) (int64, error) {
	aliasPath := filepath.Join(versionDir, latestFileName)
	data, err := os.ReadFile(aliasPath)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("reading latest module cache alias: %w", err)
	}
	if strings.TrimSpace(string(data)) != mod.Version {
		return 0, nil
	}
	info, err := os.Lstat(aliasPath)
	if err != nil {
		return 0, fmt.Errorf("reading latest module cache alias size: %w", err)
	}
	return info.Size(), nil
}

func removeDirIfEmpty(name string) {
	entries, err := os.ReadDir(name)
	if err == nil && len(entries) == 0 {
		_ = os.Remove(name)
	}
}

func cacheTargetNotFound(target ModuleCacheTarget) error {
	name := target.Path
	if target.Version != "" {
		name += "@" + target.Version
	}
	return fmt.Errorf("%w: %s", ErrArchiveNotFound, name)
}
