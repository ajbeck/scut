package godoc

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"golang.org/x/mod/module"
)

func TestFileArchiveStoreRoundTrip(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	want := ModuleArchive{
		Module: mod,
		Data: moduleZip(t, mod.Path, mod.Version, map[string]string{
			"go.mod":                         "module example.com/acme/tool\n",
			"sub/sub.go":                     "package sub\n",
			"internal/helper/helper.go":      "package helper\n",
			"internal/helper/helper_test.go": "package helper\n",
		}),
		Revision: "0123456789012345678901234567890123456789",
	}
	want = verifiedTestArchive(t, want)
	store := FileArchiveStore{Root: t.TempDir()}

	if err := store.Put(t.Context(), want); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	got, err := store.Get(t.Context(), mod)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Module != want.Module {
		t.Fatalf("Module = %#v, want %#v", got.Module, want.Module)
	}
	if !bytes.Equal(got.Data, want.Data) {
		t.Fatal("Data does not match stored module ZIP")
	}
	if got.Revision != want.Revision {
		t.Fatalf("Revision = %q, want %q", got.Revision, want.Revision)
	}

	if err := store.SetLatest(t.Context(), mod); err != nil {
		t.Fatalf("SetLatest() error = %v", err)
	}
	latest, err := store.Latest(t.Context(), mod.Path)
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if latest != mod {
		t.Fatalf("Latest() = %#v, want %#v", latest, mod)
	}
}

func TestFileArchiveStoreRejectsInvalidArchiveWithoutPublishing(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	store := FileArchiveStore{Root: t.TempDir()}
	archive := ModuleArchive{
		Module: mod,
		Data: moduleZip(t, "example.com/wrong/root", mod.Version, map[string]string{
			"tool.go": "package tool\n",
		}),
	}
	archive.Verification = ArchiveVerification{Hash: "h1:invalid", Source: verificationPolicyOff}

	if err := store.Put(t.Context(), archive); err == nil {
		t.Fatal("Put() error = nil, want invalid archive error")
	}
	if _, err := store.Get(t.Context(), mod); !errors.Is(err, ErrArchiveNotFound) {
		t.Fatalf("Get() error = %v, want ErrArchiveNotFound", err)
	}
}

func TestFileArchiveStoreRequiresVerificationBeforePublishing(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	store := FileArchiveStore{Root: t.TempDir()}
	archive := ModuleArchive{
		Module: mod,
		Data: moduleZip(t, mod.Path, mod.Version, map[string]string{
			"tool.go": "package tool\n",
		}),
	}

	if err := store.Put(t.Context(), archive); err == nil || !strings.Contains(err.Error(), "verification") {
		t.Fatalf("Put() error = %v, want missing verification error", err)
	}
	if _, err := store.Get(t.Context(), mod); !errors.Is(err, ErrArchiveNotFound) {
		t.Fatalf("Get() error = %v, want ErrArchiveNotFound", err)
	}
}

func TestFileArchiveStorePublishesConcurrentWritesAtomically(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	archive := ModuleArchive{
		Module: mod,
		Data: moduleZip(t, mod.Path, mod.Version, map[string]string{
			"tool.go": "package tool\n",
		}),
	}
	archive = verifiedTestArchive(t, archive)
	store := FileArchiveStore{Root: t.TempDir()}
	start := make(chan struct{})
	done := make(chan struct{})
	errs := make(chan error, 32)
	var writers sync.WaitGroup

	for range 16 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			<-start
			if err := store.Put(context.Background(), archive); err != nil {
				errs <- err
			}
		}()
	}
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			select {
			case <-done:
				return
			default:
				_, err := store.Get(context.Background(), mod)
				if err != nil && !errors.Is(err, ErrArchiveNotFound) {
					errs <- err
					return
				}
				runtime.Gosched()
			}
		}
	}()

	close(start)
	writers.Wait()
	close(done)
	<-readerDone
	close(errs)
	for err := range errs {
		t.Errorf("concurrent cache operation failed: %v", err)
	}
	if t.Failed() {
		return
	}
	if _, err := store.Get(t.Context(), mod); err != nil {
		t.Fatalf("Get() after concurrent writes error = %v", err)
	}

	entryDir, err := store.entryDir(mod)
	if err != nil {
		t.Fatalf("entryDir() error = %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(entryDir))
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".archive-") {
			t.Fatalf("temporary cache entry remains after publication: %s", entry.Name())
		}
	}
}

func verifiedTestArchive(t *testing.T, archive ModuleArchive) ModuleArchive {
	t.Helper()
	verification, err := (ModuleArchiveVerifier{Policy: ModuleDownloadPolicy{GOSUMDB: "off"}}).Verify(t.Context(), archive)
	if err != nil {
		t.Fatalf("Verify(%s) error = %v", archive.Module, err)
	}
	archive.Verification = verification
	return archive
}

type failingArchiveStore struct {
	err error
}

func (s failingArchiveStore) Get(context.Context, module.Version) (ModuleArchive, error) {
	return ModuleArchive{}, s.err
}

func (s failingArchiveStore) Put(context.Context, ModuleArchive) error {
	return s.err
}

func (s failingArchiveStore) Latest(context.Context, string) (module.Version, error) {
	return module.Version{}, s.err
}

func (s failingArchiveStore) SetLatest(context.Context, module.Version) error {
	return s.err
}

func TestFileArchiveStoreHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	store := FileArchiveStore{Root: t.TempDir()}

	if _, err := store.Get(ctx, mod); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get() error = %v, want context.Canceled", err)
	}
	if err := store.Put(ctx, ModuleArchive{Module: mod, Data: []byte("zip")}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Put() error = %v, want context.Canceled", err)
	}
}
