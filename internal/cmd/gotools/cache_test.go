//go:build goexperiment.jsonv2

package gotools

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	json "encoding/json/v2"

	"github.com/ajbeck/scut/internal/godoc"
	"golang.org/x/mod/module"
)

func TestCachePathCmdPrintsOwnedCacheRoot(t *testing.T) {
	store := useTestModuleCache(t)
	var stdout bytes.Buffer

	if err := new(cachePathCmd).Run(&stdout); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := stdout.String(), store.Root+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestCacheListCmdEmitsFilteredJSON(t *testing.T) {
	store := useTestModuleCache(t)
	selected := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	other := module.Version{Path: "example.com/other", Version: "v1.0.0"}
	putCommandTestArchive(t, store, selected)
	putCommandTestArchive(t, store, other)
	if err := store.SetLatest(t.Context(), selected); err != nil {
		t.Fatalf("SetLatest() error = %v", err)
	}
	var stdout bytes.Buffer

	if err := (&cacheListCmd{Target: selected.Path, JSON: true}).Run(&stdout); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	var out cacheListOutput
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &out); err != nil {
		t.Fatalf("Unmarshal() error = %v\n%s", err, stdout.String())
	}
	if out.Path != store.Root || out.Size == 0 {
		t.Fatalf("output path/size = %q/%d, want %q/non-zero", out.Path, out.Size, store.Root)
	}
	if got, want := len(out.Entries), 1; got != want {
		t.Fatalf("len(Entries) = %d, want %d", got, want)
	}
	entry := out.Entries[0]
	if entry.Module != selected.Path || entry.Version != selected.Version || !entry.Latest || entry.Status != "valid" {
		t.Fatalf("entry = %#v, want selected valid latest entry", entry)
	}
	if out.Problems == nil {
		t.Fatal("Problems = nil, want an empty JSON array")
	}
}

func TestCacheVerifyCmdWritesJSONBeforeReturningInvalidStatus(t *testing.T) {
	store := useTestModuleCache(t)
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	putCommandTestArchive(t, store, mod)
	inventory, err := store.InspectCache(t.Context())
	if err != nil {
		t.Fatalf("InspectCache() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(inventory.Entries[0].Path, "module.ziphash"), []byte("h1:wrong\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(hash) error = %v", err)
	}
	var stdout bytes.Buffer

	err = (&cacheVerifyCmd{JSON: true}).Run(&stdout)
	var verificationErr cacheVerificationError
	if !errors.As(err, &verificationErr) {
		t.Fatalf("Run() error = %v, want cacheVerificationError", err)
	}
	var out cacheVerifyOutput
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &out); err != nil {
		t.Fatalf("Unmarshal() error = %v\n%s", err, stdout.String())
	}
	if out.Valid || out.Checked != 1 || len(out.Problems) != 1 {
		t.Fatalf("output = %#v, want one invalid entry", out)
	}
}

func TestCacheRemoveCmdRemovesExactVersionAndEmitsJSON(t *testing.T) {
	store := useTestModuleCache(t)
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	putCommandTestArchive(t, store, mod)
	var stdout bytes.Buffer

	if err := (&cacheRemoveCmd{Target: mod.Path + "@" + mod.Version, JSON: true}).Run(&stdout); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	var out cacheMutationOutput
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &out); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if out.Path != store.Root || out.Removed != 1 || out.BytesFreed == 0 {
		t.Fatalf("output = %#v, want one removed entry", out)
	}
	if _, err := store.Get(t.Context(), mod); !errors.Is(err, godoc.ErrArchiveNotFound) {
		t.Fatalf("Get() error = %v, want ErrArchiveNotFound", err)
	}
}

func TestCacheCleanCmdIsIdempotent(t *testing.T) {
	store := useTestModuleCache(t)
	putCommandTestArchive(t, store, module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"})
	for run, wantRemoved := range []int{1, 0} {
		var stdout bytes.Buffer
		if err := (&cacheCleanCmd{JSON: true}).Run(&stdout); err != nil {
			t.Fatalf("Run(%d) error = %v", run, err)
		}
		var out cacheMutationOutput
		if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &out); err != nil {
			t.Fatalf("Unmarshal(%d) error = %v", run, err)
		}
		if out.Removed != wantRemoved {
			t.Fatalf("Run(%d) Removed = %d, want %d", run, out.Removed, wantRemoved)
		}
	}
}

func TestCachePruneCmdRequiresPolicyAndAcceptsHumanSize(t *testing.T) {
	useTestModuleCache(t)
	if err := new(cachePruneCmd).Run(&bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "requires") {
		t.Fatalf("Run() error = %v, want required policy error", err)
	}
	cmd := &cachePruneCmd{MaxSize: "1.5KiB"}
	policy, err := cmd.policy()
	if err != nil {
		t.Fatalf("policy() error = %v", err)
	}
	if policy.MaxSize == nil || *policy.MaxSize != 1536 {
		t.Fatalf("MaxSize = %v, want 1536", policy.MaxSize)
	}
	for _, invalid := range []string{"", "-1", "1XB", "."} {
		if invalid == "" {
			continue
		}
		if _, err := parseByteSize(invalid); err == nil {
			t.Fatalf("parseByteSize(%q) error = nil", invalid)
		}
	}
}

func useTestModuleCache(t *testing.T) godoc.FileArchiveStore {
	t.Helper()
	store := godoc.FileArchiveStore{Root: filepath.Join(t.TempDir(), "modules")}
	original := newModuleCacheStore
	newModuleCacheStore = func() (godoc.FileArchiveStore, error) { return store, nil }
	t.Cleanup(func() { newModuleCacheStore = original })
	return store
}

func putCommandTestArchive(t *testing.T, store godoc.FileArchiveStore, mod module.Version) {
	t.Helper()
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	w, err := zw.Create(path.Join(mod.Path+"@"+mod.Version, "tool.go"))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := w.Write([]byte("package tool\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := store.Put(t.Context(), godoc.ModuleArchive{Module: mod, Data: archive.Bytes()}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
}
