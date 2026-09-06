package godoc

import (
	"go/build"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/afero"
	"golang.org/x/mod/module"
)

func TestDocumentationPipelineCachesPublicModuleAcrossConcurrentAndOfflineLookups(t *testing.T) {
	const (
		modulePath  = "example.com/acme/tool"
		version     = "v1.2.3"
		packagePath = modulePath + "/widget"
	)
	proxy := newModuleProxyServer(t, modulePath, version, map[string]string{
		"go.mod": "module " + modulePath + "\n\ngo 1.26\n",
		"widget/common.go": `// Package widget exercises the complete module pipeline.
package widget

const Common = true
`,
		"widget/platform_linux.go": `package widget

const Linux = true
`,
		"widget/platform_windows.go": `package conflicting

const Windows = true
`,
		"widget/widget_test.go": `package widget_test

const TestOnly = true
`,
		"internal/helper/helper.go": "package helper\n",
	})
	t.Cleanup(proxy.Close)

	store := FileArchiveStore{Root: t.TempDir()}
	context := build.Default
	context.GOOS = "linux"
	context.GOARCH = "amd64"
	context.CgoEnabled = false
	const workers = 8
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			client := testPipelineClient(store, proxy.URL, proxy.Client(), SourceBuildContext{Context: context})
			out, err := client.Doc(t.Context(), Options{Package: packagePath, Version: version})
			if err == nil && (!strings.Contains(out, "Common") || !strings.Contains(out, "Linux") || strings.Contains(out, "Windows")) {
				err = &unexpectedDocumentationError{output: out}
			}
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Doc() error = %v", err)
		}
	}

	archive, err := store.Get(t.Context(), module.Version{Path: modulePath, Version: version})
	if err != nil {
		t.Fatalf("Get() after concurrent lookups error = %v", err)
	}
	if len(archive.Data) == 0 {
		t.Fatal("cached archive is empty")
	}

	proxy.Close()
	offline := testPipelineClient(store, "off", nil, SourceBuildContext{Context: context})
	out, err := offline.Doc(t.Context(), Options{Package: packagePath, Version: version})
	if err != nil {
		t.Fatalf("offline Doc() error = %v", err)
	}
	if !strings.Contains(out, "complete module pipeline") {
		t.Fatalf("offline Doc() output missing package documentation:\n%s", out)
	}
}

func TestDocumentationPipelineRejectsCorruptOwnedCacheWithoutRemoteFallback(t *testing.T) {
	const (
		modulePath = "example.com/acme/tool"
		version    = "v1.2.3"
	)
	proxy := newModuleProxyServer(t, modulePath, version, map[string]string{
		"go.mod":        "module " + modulePath + "\n",
		"widget/doc.go": "package widget\n",
	})
	t.Cleanup(proxy.Close)
	store := FileArchiveStore{Root: t.TempDir()}
	client := testPipelineClient(store, proxy.URL, proxy.Client(), SourceBuildContext{})
	if _, err := client.Doc(t.Context(), Options{Package: modulePath + "/widget", Version: version}); err != nil {
		t.Fatalf("initial Doc() error = %v", err)
	}
	proxy.Close()

	entryDir, err := store.entryDir(module.Version{Path: modulePath, Version: version})
	if err != nil {
		t.Fatalf("entryDir() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(entryDir, archiveFileName), []byte("corrupt"), 0o644); err != nil {
		t.Fatalf("corrupting cached ZIP: %v", err)
	}

	offline := testPipelineClient(store, "off", nil, SourceBuildContext{})
	_, err = offline.Doc(t.Context(), Options{Package: modulePath + "/widget", Version: version})
	if err == nil {
		t.Fatal("Doc() error = nil, want corrupt cache error")
	}
	if !strings.Contains(err.Error(), "validating cached module archive") {
		t.Fatalf("Doc() error = %v, want cache validation failure", err)
	}
	if strings.Contains(err.Error(), "GOPROXY=off") {
		t.Fatalf("Doc() fell through corrupt cache to remote policy: %v", err)
	}
}

func TestDocumentationPipelineCachesPrivateGitModule(t *testing.T) {
	const (
		modulePath  = "github.com/private/tool"
		version     = "v1.2.3"
		packagePath = modulePath + "/widget"
	)
	repo := afero.NewMemMapFs()
	writeTestFile(t, repo, "/go.mod", []byte("module "+modulePath+"\n\ngo 1.26\n"))
	writeTestFile(t, repo, "/widget/common.go", []byte("// Package widget comes from private Git.\npackage widget\n\nconst Common = true\n"))
	writeTestFile(t, repo, "/widget/platform_linux.go", []byte("package widget\n\nconst Linux = true\n"))
	writeTestFile(t, repo, "/widget/platform_windows.go", []byte("package conflicting\n\nconst Windows = true\n"))
	writeTestFile(t, repo, "/internal/helper/helper.go", []byte("package helper\n"))
	store := FileArchiveStore{Root: t.TempDir()}
	cloner := &fakeGitCloner{
		fs:       repo,
		revision: "0123456789012345678901234567890123456789",
	}
	policy := ModuleDownloadPolicy{
		GOPROXY:   "direct",
		GOPRIVATE: "github.com/private/*",
		GONOSUMDB: "github.com/private/*",
		GOSUMDB:   defaultGOSUMDB,
	}
	verifier := ModuleArchiveVerifier{Policy: policy}
	context := build.Default
	context.GOOS = "linux"
	context.GOARCH = "amd64"
	client := Client{
		Resolver: Resolver{Fetchers: []SourceFetcher{
			ArchiveFetcher{Store: store, Verifier: verifier},
			RemoteFetcher{
				Policy:   policy,
				Verifier: verifier,
				Direct: GitFetcher{
					Store:        store,
					Cloner:       cloner,
					AuthProvider: nilGitAuthProvider{},
				},
			},
		}},
		BuildContext: SourceBuildContext{Context: context},
	}

	out, err := client.Doc(t.Context(), Options{Package: packagePath, Version: version})
	if err != nil {
		t.Fatalf("Doc() error = %v", err)
	}
	if !strings.Contains(out, "private Git") || !strings.Contains(out, "Linux") || strings.Contains(out, "Windows") {
		t.Fatalf("Doc() output does not match active private Git source:\n%s", out)
	}
	if got := len(cloner.requests); got != 1 {
		t.Fatalf("clone requests = %d, want 1", got)
	}

	offline := Client{
		Resolver: Resolver{Fetchers: []SourceFetcher{
			ArchiveFetcher{Store: store, Verifier: verifier},
			RemoteFetcher{Policy: ModuleDownloadPolicy{GOPROXY: "off"}},
		}},
		BuildContext: SourceBuildContext{Context: context},
	}
	if _, err := offline.Doc(t.Context(), Options{Package: packagePath, Version: version}); err != nil {
		t.Fatalf("offline private Doc() error = %v", err)
	}
	if got := len(cloner.requests); got != 1 {
		t.Fatalf("clone requests after offline lookup = %d, want 1", got)
	}
}

func testPipelineClient(store ArchiveStore, proxyURL string, client *http.Client, buildContext SourceBuildContext) Client {
	policy := ModuleDownloadPolicy{GOPROXY: proxyURL, GOSUMDB: "off"}
	verifier := ModuleArchiveVerifier{Policy: policy}
	return Client{
		Resolver: Resolver{Fetchers: []SourceFetcher{
			ArchiveFetcher{Store: store, Verifier: verifier},
			RemoteFetcher{
				Policy:   policy,
				Verifier: verifier,
				Proxy: ProxyFetcher{
					Client:       client,
					Store:        store,
					DiscoveryURL: func(string) string { return proxyURL },
				},
			},
		}},
		BuildContext: buildContext,
	}
}

type unexpectedDocumentationError struct {
	output string
}

func (e *unexpectedDocumentationError) Error() string {
	return "documentation did not match active build context:\n" + e.output
}
