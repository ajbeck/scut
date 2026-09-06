package godoc

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/mod/module"
)

func TestParseModuleCacheTarget(t *testing.T) {
	tests := []struct {
		raw     string
		want    ModuleCacheTarget
		wantErr bool
	}{
		{raw: "example.com/acme/tool", want: ModuleCacheTarget{Path: "example.com/acme/tool"}},
		{raw: "example.com/acme/tool@v1.2.3", want: ModuleCacheTarget{Path: "example.com/acme/tool", Version: "v1.2.3"}},
		{raw: "", wantErr: true},
		{raw: "../outside", wantErr: true},
		{raw: "example.com/acme/tool@latest", wantErr: true},
		{raw: "example.com/acme/tool@v1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := ParseModuleCacheTarget(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseModuleCacheTarget(%q) error = nil", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseModuleCacheTarget(%q) error = %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("ParseModuleCacheTarget(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestFileArchiveStoreInspectCacheVerifiesSelfHash(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	store := FileArchiveStore{Root: t.TempDir()}
	putTestArchive(t, store, mod, "package tool\n")
	if err := store.SetLatest(t.Context(), mod); err != nil {
		t.Fatalf("SetLatest() error = %v", err)
	}

	inventory, err := store.InspectCache(t.Context())
	if err != nil {
		t.Fatalf("InspectCache() error = %v", err)
	}
	if got, want := len(inventory.Entries), 1; got != want {
		t.Fatalf("len(Entries) = %d, want %d", got, want)
	}
	entry := inventory.Entries[0]
	if entry.Status != ModuleCacheEntryValid || !entry.Latest {
		t.Fatalf("entry = %#v, want valid latest entry", entry)
	}
	if entry.Size == 0 || inventory.Size != entry.Size {
		t.Fatalf("inventory size = %d, entry size = %d", inventory.Size, entry.Size)
	}

	if err := os.WriteFile(filepath.Join(entry.Path, archiveHashName), []byte("h1:wrong\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(hash) error = %v", err)
	}
	if _, err := store.Get(t.Context(), mod); err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("Get() error = %v, want hash mismatch", err)
	}
	inventory, err = store.InspectCache(t.Context())
	if err != nil {
		t.Fatalf("InspectCache() error = %v", err)
	}
	if got := inventory.Entries[0].Status; got != ModuleCacheEntryInvalid {
		t.Fatalf("Status = %q, want %q", got, ModuleCacheEntryInvalid)
	}
}

func TestFileArchiveStoreInspectCacheReportsLegacyAndLayoutProblems(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	store := FileArchiveStore{Root: t.TempDir()}
	putTestArchive(t, store, mod, "package tool\n")
	entryDir, err := store.entryDir(mod)
	if err != nil {
		t.Fatalf("entryDir() error = %v", err)
	}
	if err := os.Remove(filepath.Join(entryDir, archiveHashName)); err != nil {
		t.Fatalf("Remove(hash) error = %v", err)
	}
	versionDir := filepath.Dir(entryDir)
	if err := os.Mkdir(filepath.Join(versionDir, ".archive-abandoned"), 0o755); err != nil {
		t.Fatalf("Mkdir(temp entry) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, latestFileName), []byte("v1.9.0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(latest) error = %v", err)
	}

	inventory, err := store.InspectCache(t.Context())
	if err != nil {
		t.Fatalf("InspectCache() error = %v", err)
	}
	if got := inventory.Entries[0].Status; got != ModuleCacheEntryIncomplete {
		t.Fatalf("Status = %q, want %q", got, ModuleCacheEntryIncomplete)
	}
	if got, want := len(inventory.Problems), 2; got != want {
		t.Fatalf("len(Problems) = %d, want %d: %#v", got, want, inventory.Problems)
	}
}

func TestFileArchiveStoreInspectReportsMissingVerificationAsIncomplete(t *testing.T) {
	store := FileArchiveStore{Root: t.TempDir()}
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	putTestArchive(t, store, mod, "package tool\n")
	entryDir, err := store.entryDir(mod)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(entryDir, verificationName)); err != nil {
		t.Fatal(err)
	}

	inventory, err := store.InspectCache(t.Context())
	if err != nil {
		t.Fatalf("InspectCache() error = %v", err)
	}
	if got := inventory.Entries[0].Status; got != ModuleCacheEntryIncomplete {
		t.Fatalf("Status = %q, want %q", got, ModuleCacheEntryIncomplete)
	}
	if !strings.Contains(inventory.Entries[0].Problem, "verification is missing") {
		t.Fatalf("Problem = %q, want missing verification", inventory.Entries[0].Problem)
	}
}

func TestFileArchiveStoreRemoveCacheVersionClearsOnlyMatchingLatest(t *testing.T) {
	store := FileArchiveStore{Root: t.TempDir()}
	selected := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	remaining := module.Version{Path: selected.Path, Version: "v1.4.0"}
	for _, mod := range []module.Version{selected, remaining} {
		putTestArchive(t, store, mod, "package tool\n")
	}
	if err := store.SetLatest(t.Context(), selected); err != nil {
		t.Fatalf("SetLatest() error = %v", err)
	}

	removed, err := store.RemoveCache(t.Context(), ModuleCacheTarget{Path: selected.Path, Version: selected.Version})
	if err != nil {
		t.Fatalf("RemoveCache() error = %v", err)
	}
	if removed.Entries != 1 || removed.Bytes == 0 {
		t.Fatalf("RemoveCache() = %#v, want one non-empty entry", removed)
	}
	if _, err := store.Get(t.Context(), selected); !errors.Is(err, ErrArchiveNotFound) {
		t.Fatalf("Get(selected) error = %v, want ErrArchiveNotFound", err)
	}
	if _, err := store.Get(t.Context(), remaining); err != nil {
		t.Fatalf("Get(remaining) error = %v", err)
	}
	if _, err := store.Latest(t.Context(), selected.Path); !errors.Is(err, ErrArchiveNotFound) {
		t.Fatalf("Latest() error = %v, want ErrArchiveNotFound", err)
	}
}

func TestFileArchiveStoreRemoveModulePreservesNestedModule(t *testing.T) {
	store := FileArchiveStore{Root: t.TempDir()}
	parent := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	nested := module.Version{Path: "example.com/acme/tool/plugin", Version: "v1.0.0"}
	putTestArchive(t, store, parent, "package tool\n")
	putTestArchive(t, store, nested, "package plugin\n")

	if _, err := store.RemoveCache(t.Context(), ModuleCacheTarget{Path: parent.Path}); err != nil {
		t.Fatalf("RemoveCache(parent) error = %v", err)
	}
	if _, err := store.Get(t.Context(), parent); !errors.Is(err, ErrArchiveNotFound) {
		t.Fatalf("Get(parent) error = %v, want ErrArchiveNotFound", err)
	}
	if _, err := store.Get(t.Context(), nested); err != nil {
		t.Fatalf("Get(nested) error = %v", err)
	}
}

func TestFileArchiveStoreCleanCachePreservesSibling(t *testing.T) {
	parent := t.TempDir()
	store := FileArchiveStore{Root: filepath.Join(parent, "modules")}
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	putTestArchive(t, store, mod, "package tool\n")
	sibling := filepath.Join(parent, "keep")
	if err := os.WriteFile(sibling, []byte("keep"), 0o644); err != nil {
		t.Fatalf("WriteFile(sibling) error = %v", err)
	}

	removed, err := store.CleanCache(t.Context())
	if err != nil {
		t.Fatalf("CleanCache() error = %v", err)
	}
	if removed.Entries != 1 || removed.Bytes == 0 {
		t.Fatalf("CleanCache() = %#v, want one non-empty entry", removed)
	}
	if _, err := os.Stat(store.Root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(cache root) error = %v, want not exist", err)
	}
	if got, err := os.ReadFile(sibling); err != nil || string(got) != "keep" {
		t.Fatalf("sibling = %q, %v", got, err)
	}
	removed, err = store.CleanCache(t.Context())
	if err != nil || removed != (ModuleCacheRemoval{}) {
		t.Fatalf("second CleanCache() = %#v, %v", removed, err)
	}
}

func TestFileArchiveStoreRemoveCacheRejectsIntermediateSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not generally available to unprivileged Windows tests")
	}
	parent := t.TempDir()
	store := FileArchiveStore{Root: filepath.Join(parent, "modules")}
	external := filepath.Join(parent, "external")
	victimDir := filepath.Join(external, "acme", "tool", "@v")
	if err := os.MkdirAll(victimDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(victim) error = %v", err)
	}
	victim := filepath.Join(victimDir, "keep")
	if err := os.WriteFile(victim, []byte("keep"), 0o644); err != nil {
		t.Fatalf("WriteFile(victim) error = %v", err)
	}
	if err := os.MkdirAll(store.Root, 0o755); err != nil {
		t.Fatalf("MkdirAll(root) error = %v", err)
	}
	if err := os.Symlink(external, filepath.Join(store.Root, "example.com")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	_, err := store.RemoveCache(t.Context(), ModuleCacheTarget{Path: "example.com/acme/tool"})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("RemoveCache() error = %v, want symlink rejection", err)
	}
	if got, err := os.ReadFile(victim); err != nil || string(got) != "keep" {
		t.Fatalf("external victim = %q, %v", got, err)
	}
}

func TestFileArchiveStorePruneCacheAppliesAgeThenSize(t *testing.T) {
	store := FileArchiveStore{Root: t.TempDir()}
	now := time.Date(2026, time.September, 6, 12, 0, 0, 0, time.UTC)
	mods := []module.Version{
		{Path: "example.com/acme/tool", Version: "v1.0.0"},
		{Path: "example.com/acme/tool", Version: "v1.1.0"},
		{Path: "example.com/acme/tool", Version: "v1.2.0"},
	}
	for i, mod := range mods {
		putTestArchive(t, store, mod, "package tool\n\nvar Data = \""+strings.Repeat("x", i+1)+"\"\n")
		entryDir, err := store.entryDir(mod)
		if err != nil {
			t.Fatalf("entryDir() error = %v", err)
		}
		setTreeModTime(t, entryDir, now.Add(time.Duration(i-3)*24*time.Hour))
	}
	inventory, err := store.InspectCache(t.Context())
	if err != nil {
		t.Fatalf("InspectCache() error = %v", err)
	}
	newestSize := inventory.Entries[2].Size
	olderThan := 48 * time.Hour
	removed, err := store.PruneCache(t.Context(), ModuleCachePrunePolicy{
		OlderThan: &olderThan,
		MaxSize:   &newestSize,
		Now:       now,
	})
	if err != nil {
		t.Fatalf("PruneCache() error = %v", err)
	}
	if got, want := removed.Entries, 2; got != want {
		t.Fatalf("removed entries = %d, want %d", got, want)
	}
	for _, mod := range mods[:2] {
		if _, err := store.Get(t.Context(), mod); !errors.Is(err, ErrArchiveNotFound) {
			t.Fatalf("Get(%s) error = %v, want ErrArchiveNotFound", mod.Version, err)
		}
	}
	if _, err := store.Get(t.Context(), mods[2]); err != nil {
		t.Fatalf("Get(newest) error = %v", err)
	}
}

func TestFileArchiveStorePruneCacheReportsUnassociatedBytes(t *testing.T) {
	store := FileArchiveStore{Root: t.TempDir()}
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	putTestArchive(t, store, mod, "package tool\n")
	orphan := filepath.Join(store.Root, "orphan")
	if err := os.WriteFile(orphan, []byte("unassociated"), 0o644); err != nil {
		t.Fatalf("WriteFile(orphan) error = %v", err)
	}
	maxSize := int64(0)

	removed, err := store.PruneCache(t.Context(), ModuleCachePrunePolicy{MaxSize: &maxSize})
	var sizeErr ModuleCacheSizeError
	if !errors.As(err, &sizeErr) {
		t.Fatalf("PruneCache() error = %v, want ModuleCacheSizeError", err)
	}
	if removed.Entries != 1 || removed.Bytes == 0 {
		t.Fatalf("PruneCache() removal = %#v, want one entry", removed)
	}
	if got, err := os.ReadFile(orphan); err != nil || string(got) != "unassociated" {
		t.Fatalf("orphan = %q, %v; prune must not guess how to remove it", got, err)
	}
}

func putTestArchive(t *testing.T, store FileArchiveStore, mod module.Version, source string) {
	t.Helper()
	archive := ModuleArchive{
		Module: mod,
		Data:   moduleZip(t, mod.Path, mod.Version, map[string]string{"tool.go": source}),
	}
	archive = verifiedTestArchive(t, archive)
	if err := store.Put(t.Context(), archive); err != nil {
		t.Fatalf("Put(%s@%s) error = %v", mod.Path, mod.Version, err)
	}
}

func setTreeModTime(t *testing.T, root string, modified time.Time) {
	t.Helper()
	err := filepath.Walk(root, func(name string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(name, modified, modified)
	})
	if err != nil {
		t.Fatalf("setting modification time: %v", err)
	}
}
