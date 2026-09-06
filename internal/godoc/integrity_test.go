package godoc

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb"
	"golang.org/x/mod/sumdb/note"
)

func TestModuleArchiveVerifierUsesApplicableGoSum(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	archive := ModuleArchive{
		Module: mod,
		Data:   moduleZip(t, mod.Path, mod.Version, map[string]string{"tool.go": "package tool\n"}),
	}
	hash, err := hashModuleArchive(archive)
	if err != nil {
		t.Fatal(err)
	}
	fs := afero.NewMemMapFs()
	if err := afero.WriteFile(fs, "/go.sum", []byte(mod.Path+" "+mod.Version+" "+hash+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	database := &recordingChecksumDatabase{}

	verification, err := (ModuleArchiveVerifier{
		Policy: ModuleDownloadPolicy{GOSUMDB: defaultGOSUMDB},
		GoSums: GoSumFiles{FS: fs, Paths: []string{"/go.sum"}},
		SumDB:  database,
	}).Verify(t.Context(), archive)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if got, want := verification, (ArchiveVerification{Hash: hash, Source: verificationGoSum}); got != want {
		t.Fatalf("verification = %#v, want %#v", got, want)
	}
	if database.calls != 0 {
		t.Fatalf("checksum database calls = %d, want 0", database.calls)
	}
}

func TestModuleArchiveVerifierRejectsGoSumMismatchWithoutDatabaseFallback(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	archive := ModuleArchive{
		Module: mod,
		Data:   moduleZip(t, mod.Path, mod.Version, map[string]string{"tool.go": "package tool\n"}),
	}
	fs := afero.NewMemMapFs()
	if err := afero.WriteFile(fs, "/go.sum", []byte(mod.Path+" "+mod.Version+" h1:wrong\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	database := &recordingChecksumDatabase{}

	_, err := (ModuleArchiveVerifier{
		Policy: ModuleDownloadPolicy{GOSUMDB: defaultGOSUMDB},
		GoSums: GoSumFiles{FS: fs, Paths: []string{"/go.sum"}},
		SumDB:  database,
	}).Verify(t.Context(), archive)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("Verify() error = %v, want checksum mismatch", err)
	}
	if database.calls != 0 {
		t.Fatalf("checksum database calls = %d, want 0", database.calls)
	}
}

func TestModuleArchiveVerifierUsesChecksumDatabase(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	archive := ModuleArchive{
		Module: mod,
		Data:   moduleZip(t, mod.Path, mod.Version, map[string]string{"tool.go": "package tool\n"}),
	}
	hash, err := hashModuleArchive(archive)
	if err != nil {
		t.Fatal(err)
	}
	database := &recordingChecksumDatabase{
		name:  "sum.example.com",
		lines: []string{mod.Path + " " + mod.Version + " " + hash},
	}

	verification, err := (ModuleArchiveVerifier{
		Policy: ModuleDownloadPolicy{GOSUMDB: "sum.example.com"},
		SumDB:  database,
	}).Verify(t.Context(), archive)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if got, want := verification.Source, "sumdb:sum.example.com"; got != want {
		t.Fatalf("Source = %q, want %q", got, want)
	}
}

func TestModuleArchiveVerifierHonorsChecksumPolicyOptOuts(t *testing.T) {
	mod := module.Version{Path: "private.example.com/acme/tool", Version: "v1.2.3"}
	archive := ModuleArchive{
		Module: mod,
		Data:   moduleZip(t, mod.Path, mod.Version, map[string]string{"tool.go": "package tool\n"}),
	}
	for _, test := range []struct {
		name   string
		policy ModuleDownloadPolicy
		source string
	}{
		{name: "GOSUMDB off", policy: ModuleDownloadPolicy{GOSUMDB: "off"}, source: verificationPolicyOff},
		{name: "GONOSUMDB match", policy: ModuleDownloadPolicy{GOSUMDB: defaultGOSUMDB, GONOSUMDB: "private.example.com"}, source: verificationPolicySkip},
	} {
		t.Run(test.name, func(t *testing.T) {
			database := &recordingChecksumDatabase{err: errors.New("must not be called")}
			verification, err := (ModuleArchiveVerifier{Policy: test.policy, SumDB: database}).Verify(t.Context(), archive)
			if err != nil {
				t.Fatalf("Verify() error = %v", err)
			}
			if verification.Source != test.source {
				t.Fatalf("Source = %q, want %q", verification.Source, test.source)
			}
			if database.calls != 0 {
				t.Fatalf("checksum database calls = %d, want 0", database.calls)
			}
		})
	}
}

func TestModuleArchiveVerifierReusesStoredPublicVerificationOffline(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	archive := ModuleArchive{
		Module: mod,
		Data:   moduleZip(t, mod.Path, mod.Version, map[string]string{"tool.go": "package tool\n"}),
	}
	hash, err := hashModuleArchive(archive)
	if err != nil {
		t.Fatal(err)
	}
	archive.Verification = ArchiveVerification{Hash: hash, Source: "sumdb:sum.golang.org"}
	database := &recordingChecksumDatabase{err: errors.New("offline")}

	verification, err := (ModuleArchiveVerifier{
		Policy: ModuleDownloadPolicy{GOSUMDB: defaultGOSUMDB},
		SumDB:  database,
	}).Verify(t.Context(), archive)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if verification != archive.Verification {
		t.Fatalf("verification = %#v, want stored %#v", verification, archive.Verification)
	}
	if database.calls != 0 {
		t.Fatalf("checksum database calls = %d, want 0", database.calls)
	}
}

func TestArchiveFetcherRejectsCurrentGoSumMismatch(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	store := FileArchiveStore{Root: t.TempDir()}
	archive := verifiedTestArchive(t, ModuleArchive{
		Module: mod,
		Data:   moduleZip(t, mod.Path, mod.Version, map[string]string{"tool.go": "package tool\n"}),
	})
	if err := store.Put(t.Context(), archive); err != nil {
		t.Fatal(err)
	}
	fs := afero.NewMemMapFs()
	if err := afero.WriteFile(fs, "/go.sum", []byte(mod.Path+" "+mod.Version+" h1:wrong\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fetcher := ArchiveFetcher{
		Store: store,
		Verifier: ModuleArchiveVerifier{
			Policy: ModuleDownloadPolicy{GOSUMDB: "off"},
			GoSums: GoSumFiles{FS: fs, Paths: []string{"/go.sum"}},
		},
	}

	_, err := fetcher.Fetch(t.Context(), mod.Path, Options{Version: mod.Version})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("Fetch() error = %v, want current go.sum mismatch", err)
	}
}

func TestModuleArchiveVerifierRejectsMalformedAndWrongRootArchives(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	for _, test := range []struct {
		name string
		data []byte
	}{
		{name: "truncated", data: []byte("PK\x03\x04")},
		{name: "wrong root", data: moduleZip(t, "example.com/wrong", mod.Version, map[string]string{"tool.go": "package tool\n"})},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := (ModuleArchiveVerifier{Policy: ModuleDownloadPolicy{GOSUMDB: "off"}}).Verify(t.Context(), ModuleArchive{Module: mod, Data: test.data})
			if err == nil {
				t.Fatal("Verify() error = nil, want structural validation error")
			}
		})
	}
}

func TestApplicableGoSumFilesIncludesWorkspaceAndMemberSums(t *testing.T) {
	fs := afero.NewMemMapFs()
	workFile := "/workspace/go.work"
	if err := fs.MkdirAll("/workspace/one", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.MkdirAll("/workspace/two", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(fs, workFile, []byte("go 1.26\n\nuse (\n\t./one\n\t./two\n)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWORK", workFile)

	got := applicableGoSumFiles(fs, "/workspace/one/pkg", "/workspace/one")
	want := []string{"/workspace/go.work.sum", "/workspace/one/go.sum", "/workspace/two/go.sum"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("applicableGoSumFiles() = %#v, want %#v", got, want)
	}
}

func TestProxyFetcherDoesNotPublishRejectedArchive(t *testing.T) {
	const (
		modPath = "example.com/acme/tool"
		version = "v1.2.3"
	)
	proxy := newModuleProxyServer(t, modPath, version, map[string]string{"tool.go": "package tool\n"})
	t.Cleanup(proxy.Close)
	store := FileArchiveStore{Root: t.TempDir()}
	fetcher := ProxyFetcher{
		Client:   proxy.Client(),
		ProxyURL: proxy.URL,
		Store:    store,
		Verifier: rejectingArchiveVerifier{err: errors.New("checksum rejected")},
	}

	_, err := fetcher.Fetch(t.Context(), modPath, Options{Version: version})
	if err == nil || !strings.Contains(err.Error(), "checksum rejected") {
		t.Fatalf("Fetch() error = %v, want checksum rejection", err)
	}
	inventory, err := store.InspectCache(t.Context())
	if err != nil {
		t.Fatalf("InspectCache() error = %v", err)
	}
	if len(inventory.Entries) != 0 || inventory.Size != 0 {
		t.Fatalf("cache inventory = %#v, want empty", inventory)
	}
}

func TestGoChecksumDatabaseAuthenticatesLookupAndPersistsCheckpoint(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	const hash = "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	signerKey, verifierKey, err := note.GenerateKey(rand.Reader, "sum.test")
	if err != nil {
		t.Fatal(err)
	}
	testServer := sumdb.NewTestServer(signerKey, func(path, version string) ([]byte, error) {
		return []byte(fmt.Sprintf("%s %s %s\n", path, version, hash)), nil
	})
	server := httptest.NewServer(sumdb.NewServer(testServer))
	t.Cleanup(server.Close)
	stateDir := t.TempDir()
	database := GoChecksumDatabase{
		Policy:   ModuleDownloadPolicy{GOSUMDB: verifierKey + " " + server.URL, GOPROXY: "off"},
		StateDir: stateDir,
	}

	name, lines, err := database.Lookup(t.Context(), mod)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if name != "sum.test" || len(lines) != 1 || !strings.HasSuffix(lines[0], " "+hash) {
		t.Fatalf("Lookup() = %q, %#v", name, lines)
	}
	checkpoint := filepath.Join(stateDir, "sum.test", "latest")
	if data, err := os.ReadFile(checkpoint); err != nil || len(data) == 0 {
		t.Fatalf("checkpoint = %q, %v; want signed state", data, err)
	}
}

func TestGoChecksumDatabaseUsesAdvertisingModuleProxy(t *testing.T) {
	mod := module.Version{Path: "example.com/acme/tool", Version: "v1.2.3"}
	const hash = "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	signerKey, verifierKey, err := note.GenerateKey(rand.Reader, "sum.test")
	if err != nil {
		t.Fatal(err)
	}
	testServer := sumdb.NewTestServer(signerKey, func(path, version string) ([]byte, error) {
		return []byte(fmt.Sprintf("%s %s %s\n", path, version, hash)), nil
	})
	sumHandler := sumdb.NewServer(testServer)
	var advertised, proxied bool
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "/sumdb/sum.test"
		if r.URL.Path == prefix+"/supported" {
			advertised = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if !strings.HasPrefix(r.URL.Path, prefix+"/") {
			http.NotFound(w, r)
			return
		}
		proxied = true
		request := r.Clone(r.Context())
		request.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		sumHandler.ServeHTTP(w, request)
	}))
	t.Cleanup(proxy.Close)
	database := GoChecksumDatabase{
		Policy: ModuleDownloadPolicy{
			GOSUMDB: verifierKey,
			GOPROXY: proxy.URL,
		},
		Client:   proxy.Client(),
		StateDir: t.TempDir(),
	}

	_, lines, err := database.Lookup(t.Context(), mod)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if !advertised || !proxied || len(lines) != 1 {
		t.Fatalf("advertised/proxied/lines = %t/%t/%#v", advertised, proxied, lines)
	}
}

func TestChecksumStateCompareAndSwapAcrossClients(t *testing.T) {
	stateDir := t.TempDir()
	first := &checksumClientOps{ctx: t.Context(), key: "key", stateDir: stateDir, cache: make(map[string][]byte), config: make(map[string][]byte)}
	second := &checksumClientOps{ctx: t.Context(), key: "key", stateDir: stateDir, cache: make(map[string][]byte), config: make(map[string][]byte)}
	const name = "sum.test/latest"

	old, err := first.ReadConfig(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.ReadConfig(name); err != nil {
		t.Fatal(err)
	}
	if err := first.WriteConfig(name, old, []byte("first")); err != nil {
		t.Fatalf("first WriteConfig() error = %v", err)
	}
	if err := second.WriteConfig(name, old, []byte("second")); !errors.Is(err, sumdb.ErrWriteConflict) {
		t.Fatalf("second WriteConfig() error = %v, want ErrWriteConflict", err)
	}
}

type recordingChecksumDatabase struct {
	name  string
	lines []string
	err   error
	calls int
}

func (d *recordingChecksumDatabase) Lookup(context.Context, module.Version) (string, []string, error) {
	d.calls++
	return d.name, d.lines, d.err
}

type rejectingArchiveVerifier struct {
	err error
}

func (v rejectingArchiveVerifier) Verify(context.Context, ModuleArchive) (ArchiveVerification, error) {
	return ArchiveVerification{}, v.err
}
