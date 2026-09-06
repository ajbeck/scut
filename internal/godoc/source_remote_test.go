package godoc

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/spf13/afero"
)

func TestParseProxyRoutesPreservesGoFallbackPolicy(t *testing.T) {
	tests := []struct {
		name    string
		policy  ModuleDownloadPolicy
		want    []proxyRoute
		wantErr bool
	}{
		{
			name:   "default",
			policy: ModuleDownloadPolicy{GOPROXY: defaultGOPROXY},
			want: []proxyRoute{
				{Kind: proxyRouteURL, URL: "https://proxy.golang.org"},
				{Kind: proxyRouteDirect},
			},
		},
		{
			name:   "comma and pipe",
			policy: ModuleDownloadPolicy{GOPROXY: "one.example,two.example|three.example"},
			want: []proxyRoute{
				{Kind: proxyRouteURL, URL: "https://one.example"},
				{Kind: proxyRouteURL, URL: "https://two.example", FallbackOnError: true},
				{Kind: proxyRouteURL, URL: "https://three.example"},
			},
		},
		{
			name:   "implicit noproxy",
			policy: ModuleDownloadPolicy{GOPROXY: "https://proxy.example,direct", GONOPROXY: "*.corp.example"},
			want: []proxyRoute{
				{Kind: proxyRouteNoProxy},
				{Kind: proxyRouteURL, URL: "https://proxy.example"},
				{Kind: proxyRouteDirect},
			},
		},
		{
			name:   "direct is terminal",
			policy: ModuleDownloadPolicy{GOPROXY: "direct,https://ignored.example"},
			want:   []proxyRoute{{Kind: proxyRouteDirect}},
		},
		{
			name:   "off is terminal",
			policy: ModuleDownloadPolicy{GOPROXY: "https://proxy.example,off|https://ignored.example"},
			want: []proxyRoute{
				{Kind: proxyRouteURL, URL: "https://proxy.example"},
				{Kind: proxyRouteOff},
			},
		},
		{
			name:    "empty entries",
			policy:  ModuleDownloadPolicy{GOPROXY: ", | "},
			wantErr: true,
		},
		{
			name:    "invalid scheme",
			policy:  ModuleDownloadPolicy{GOPROXY: "ssh://proxy.example"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseProxyRoutes(tt.policy)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseProxyRoutes() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseProxyRoutes() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestRemoteFetcherCommaFallsBackOnlyOnNotFound(t *testing.T) {
	routes := []string{}
	fetcher := RemoteFetcher{
		Policy: ModuleDownloadPolicy{GOPROXY: "https://one.example,https://two.example"},
		fetchRoute: func(_ context.Context, route proxyRoute, _ string, _ Options) (PackageSource, error) {
			routes = append(routes, route.URL)
			if len(routes) == 1 {
				return PackageSource{}, fs.ErrNotExist
			}
			return PackageSource{ImportPath: "example.com/mod"}, nil
		},
	}
	if _, err := fetcher.Fetch(t.Context(), "example.com/mod", Options{}); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if want := []string{"https://one.example", "https://two.example"}; !reflect.DeepEqual(routes, want) {
		t.Fatalf("routes = %#v, want %#v", routes, want)
	}

	routes = nil
	fetcher.fetchRoute = func(_ context.Context, route proxyRoute, _ string, _ Options) (PackageSource, error) {
		routes = append(routes, route.URL)
		return PackageSource{}, errors.New("server failed")
	}
	if _, err := fetcher.Fetch(t.Context(), "example.com/mod", Options{}); err == nil {
		t.Fatal("Fetch() error = nil")
	}
	if want := []string{"https://one.example"}; !reflect.DeepEqual(routes, want) {
		t.Fatalf("routes = %#v, want %#v", routes, want)
	}
}

func TestRemoteFetcherPipeFallsBackOnAnyError(t *testing.T) {
	var routes []string
	fetcher := RemoteFetcher{
		Policy: ModuleDownloadPolicy{GOPROXY: "https://one.example|https://two.example"},
		fetchRoute: func(_ context.Context, route proxyRoute, _ string, _ Options) (PackageSource, error) {
			routes = append(routes, route.URL)
			if len(routes) == 1 {
				return PackageSource{}, errors.New("server failed")
			}
			return PackageSource{ImportPath: "example.com/mod"}, nil
		},
	}
	if _, err := fetcher.Fetch(t.Context(), "example.com/mod", Options{}); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if want := []string{"https://one.example", "https://two.example"}; !reflect.DeepEqual(routes, want) {
		t.Fatalf("routes = %#v, want %#v", routes, want)
	}
}

func TestRemoteFetcherOffReturnsActionableError(t *testing.T) {
	_, err := (RemoteFetcher{Policy: ModuleDownloadPolicy{GOPROXY: "off"}}).Fetch(
		t.Context(), "example.com/mod", Options{},
	)
	if !errors.Is(err, errProxyOff) {
		t.Fatalf("Fetch() error = %v, want GOPROXY off", err)
	}
}

func TestRemoteFetcherPrivateModuleBypassesProxy(t *testing.T) {
	proxyRequests := 0
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyRequests++
		http.Error(w, "private module leaked to proxy", http.StatusInternalServerError)
	}))
	t.Cleanup(proxy.Close)
	repo := afero.NewMemMapFs()
	writeTestFile(t, repo, "/pkg/pkg.go", []byte("package pkg\n"))
	cloner := &fakeGitCloner{fs: repo}
	fetcher := RemoteFetcher{
		Policy: ModuleDownloadPolicy{
			GOPROXY:   proxy.URL + ",direct",
			GOPRIVATE: "github.com/private/*",
			GONOPROXY: "github.com/private/*",
		},
		Proxy: ProxyFetcher{Client: proxy.Client()},
		Direct: GitFetcher{
			AuthProvider: nilGitAuthProvider{},
			Cloner:       cloner,
		},
	}

	if _, err := fetcher.Fetch(t.Context(), "github.com/private/mod/pkg", Options{Version: "latest"}); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if proxyRequests != 0 {
		t.Fatalf("proxy requests = %d, want 0", proxyRequests)
	}
	if len(cloner.requests) != 1 {
		t.Fatalf("clone requests = %d, want 1", len(cloner.requests))
	}
}

func TestRemoteFetcherExplicitNoProxyOverrideUsesProxyForPrivateModule(t *testing.T) {
	proxy := newModuleProxyServer(t, "github.com/private/mod", "v1.2.3", map[string]string{
		"pkg/pkg.go": "package pkg\n",
	})
	t.Cleanup(proxy.Close)
	cloner := &fakeGitCloner{errors: []error{errors.New("direct must not run")}}
	fetcher := RemoteFetcher{
		Policy: ModuleDownloadPolicy{
			GOPROXY:   proxy.URL,
			GOPRIVATE: "github.com/private/*",
			GONOPROXY: "none",
		},
		Proxy:  ProxyFetcher{Client: proxy.Client()},
		Direct: GitFetcher{Cloner: cloner},
	}

	if _, err := fetcher.Fetch(t.Context(), "github.com/private/mod/pkg", Options{}); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(cloner.requests) != 0 {
		t.Fatalf("clone requests = %d, want 0", len(cloner.requests))
	}
}
