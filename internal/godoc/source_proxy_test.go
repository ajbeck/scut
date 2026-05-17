package godoc

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestProxyURLsFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want []string
	}{
		{name: "default", want: []string{"https://proxy.golang.org"}},
		{name: "single", env: "https://proxy.example.com", want: []string{"https://proxy.example.com"}},
		{name: "fallbacks", env: "https://one.example.com,https://two.example.com", want: []string{"https://one.example.com", "https://two.example.com"}},
		{name: "skips_empty_and_direct", env: "https://one.example.com,,direct|https://two.example.com", want: []string{"https://one.example.com", "https://two.example.com"}},
		{name: "off_stops", env: "https://one.example.com,off,https://two.example.com", want: []string{"https://one.example.com"}},
		{name: "off_only", env: "off"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := proxyURLsFromEnv(tt.env)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("proxyURLsFromEnv(%q) = %#v, want %#v", tt.env, got, tt.want)
			}
		})
	}
}

func TestProxyFetcherFallsBackAcrossConfiguredProxies(t *testing.T) {
	miss := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(miss.Close)
	hit := newModuleProxyServer(t, "github.com/acme/tool", "v1.2.3", map[string]string{
		"tool.go": "package tool\n",
	})
	t.Cleanup(hit.Close)
	resolver := Resolver{Fetchers: []SourceFetcher{
		ProxyFetcher{Client: hit.Client(), ProxyURL: miss.URL},
		ProxyFetcher{Client: hit.Client(), ProxyURL: hit.URL},
	}}

	source, err := resolver.Fetch(context.Background(), "github.com/acme/tool", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got, want := source.Module.Version, "v1.2.3"; got != want {
		t.Fatalf("Module.Version = %q, want %q", got, want)
	}
}

func TestProxyFetcherReturnsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fetcher := ProxyFetcher{ProxyURL: "https://proxy.example.com"}

	_, err := fetcher.Fetch(ctx, "github.com/acme/tool", Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Fetch() error = %v, want context.Canceled", err)
	}
}

func TestProxyFetcherReturnsMalformedProxyURL(t *testing.T) {
	fetcher := ProxyFetcher{ProxyURL: "://bad proxy url"}

	_, err := fetcher.Fetch(context.Background(), "github.com/acme/tool", Options{})
	if err == nil {
		t.Fatal("Fetch() error = nil, want malformed URL error")
	}
	if errors.Is(err, ErrSourceNotApplicable) {
		t.Fatalf("Fetch() error = %v, want concrete malformed URL error", err)
	}
}

func TestProxyFetcherDoesNotFallBackAfterServerError(t *testing.T) {
	serverError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(serverError.Close)
	hit := newModuleProxyServer(t, "github.com/acme/tool", "v1.2.3", map[string]string{
		"tool.go": "package tool\n",
	})
	t.Cleanup(hit.Close)
	resolver := Resolver{Fetchers: []SourceFetcher{
		ProxyFetcher{Client: hit.Client(), ProxyURL: serverError.URL},
		ProxyFetcher{Client: hit.Client(), ProxyURL: hit.URL},
	}}

	_, err := resolver.Fetch(context.Background(), "github.com/acme/tool", Options{})
	if err == nil {
		t.Fatal("Fetch() error = nil, want server error")
	}
	if errors.Is(err, ErrSourceNotApplicable) {
		t.Fatalf("Fetch() error = %v, want concrete server error", err)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("Fetch() error = %v, want 500 status", err)
	}
}

func TestProxyFetcherResolvesLatestAndExtractsPackage(t *testing.T) {
	proxy := newModuleProxyServer(t, "github.com/acme/tool", "v1.2.3", map[string]string{
		"tool.go":      "package tool\n",
		"tool_test.go": "package tool\n",
		"sub/sub.go":   "package sub\n",
	})

	fetcher := ProxyFetcher{Client: proxy.Client(), ProxyURL: proxy.URL}
	source, err := fetcher.Fetch(context.Background(), "github.com/acme/tool", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if got, want := source.ImportPath, "github.com/acme/tool"; got != want {
		t.Fatalf("ImportPath = %q, want %q", got, want)
	}
	if got, want := len(source.Files), 1; got != want {
		t.Fatalf("len(Files) = %d, want %d", got, want)
	}
	if got, want := string(source.Files[0].Data), "package tool\n"; got != want {
		t.Fatalf("Data = %q, want %q", got, want)
	}
}

func TestProxyFetcherResolvesExplicitVersion(t *testing.T) {
	proxy := newModuleProxyServer(t, "github.com/acme/tool", "v1.2.3", map[string]string{
		"tool.go": "package tool\n",
	})

	fetcher := ProxyFetcher{Client: proxy.Client(), ProxyURL: proxy.URL}
	source, err := fetcher.Fetch(context.Background(), "github.com/acme/tool", Options{Version: "v1.2.3"})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if got, want := source.Version, "v1.2.3"; got != want {
		t.Fatalf("Version = %q, want %q", got, want)
	}
}

func TestProxyFetcherProbesModulePrefixes(t *testing.T) {
	proxy := newModuleProxyServer(t, "github.com/acme/tool", "v1.2.3", map[string]string{
		"sub/sub.go": "package sub\n",
	})

	fetcher := ProxyFetcher{Client: proxy.Client(), ProxyURL: proxy.URL}
	source, err := fetcher.Fetch(context.Background(), "github.com/acme/tool/sub", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if got, want := source.Module.Path, "github.com/acme/tool"; got != want {
		t.Fatalf("Module.Path = %q, want %q", got, want)
	}
	if got, want := source.ImportPath, "github.com/acme/tool/sub"; got != want {
		t.Fatalf("ImportPath = %q, want %q", got, want)
	}
}

func TestProxyFetcherUsesMetaDiscoveryBeforePrefixProbing(t *testing.T) {
	proxy := newModuleProxyServer(t, "example.com/root", "v0.4.0", map[string]string{
		"pkg/pkg.go": "package pkg\n",
	})
	discovery := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("go-get") != "1" {
			t.Fatalf("discovery missing go-get=1: %s", r.URL.String())
		}
		fmt.Fprintln(w, `<html><head><meta name="go-import" content="example.com/root git https://example.com/root.git"></head></html>`)
	}))
	t.Cleanup(discovery.Close)

	fetcher := ProxyFetcher{
		Client:       proxy.Client(),
		ProxyURL:     proxy.URL,
		DiscoveryURL: func(string) string { return discovery.URL + "?go-get=1" },
	}
	source, err := fetcher.Fetch(context.Background(), "example.com/root/pkg", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if got, want := source.Module.Path, "example.com/root"; got != want {
		t.Fatalf("Module.Path = %q, want %q", got, want)
	}
}

func TestProxyFetcherWritesCacheWhenConfigured(t *testing.T) {
	proxy := newModuleProxyServer(t, "github.com/acme/tool", "v1.2.3", map[string]string{
		"sub/sub.go": "package sub\n",
	})
	cacheFS := afero.NewMemMapFs()

	fetcher := ProxyFetcher{
		Client:   proxy.Client(),
		ProxyURL: proxy.URL,
		CacheFS:  cacheFS,
		CacheDir: "/mod",
	}
	_, err := fetcher.Fetch(context.Background(), "github.com/acme/tool/sub", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	cacheFetcher := ModCacheFetcher{FS: cacheFS, CacheDir: "/mod"}
	source, err := cacheFetcher.Fetch(context.Background(), "github.com/acme/tool/sub", Options{})
	if err != nil {
		t.Fatalf("cache Fetch() error = %v", err)
	}
	if got, want := len(source.Files), 1; got != want {
		t.Fatalf("len(Files) = %d, want %d", got, want)
	}
}

func newModuleProxyServer(t *testing.T, modPath, version string, files map[string]string) *httptest.Server {
	t.Helper()
	zipBytes := moduleZip(t, modPath, version, files)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suffixLatest := "/@latest"
		suffixInfo := "/@v/" + version + ".info"
		suffixZip := "/@v/" + version + ".zip"
		switch {
		case strings.HasSuffix(r.URL.Path, suffixLatest):
			if modulePathFromProxyPath(r.URL.Path, suffixLatest) != modPath {
				http.NotFound(w, r)
				return
			}
			fmt.Fprintf(w, `{"Version":%q}`, version)
		case strings.HasSuffix(r.URL.Path, suffixInfo):
			if modulePathFromProxyPath(r.URL.Path, suffixInfo) != modPath {
				http.NotFound(w, r)
				return
			}
			fmt.Fprintf(w, `{"Version":%q}`, version)
		case strings.HasSuffix(r.URL.Path, suffixZip):
			if modulePathFromProxyPath(r.URL.Path, suffixZip) != modPath {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipBytes)
		default:
			http.NotFound(w, r)
		}
	}))
}

func modulePathFromProxyPath(requestPath, suffix string) string {
	return strings.TrimPrefix(strings.TrimSuffix(requestPath, suffix), "/")
}

func moduleZip(t *testing.T, modPath, version string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.Create(path.Join(modPath+"@"+version, name))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if _, err := w.Write([]byte(data)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return buf.Bytes()
}
