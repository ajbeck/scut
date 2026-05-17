package godoc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/spf13/afero"
)

func TestEnvGitAuthProviderPrefersGitHubToken(t *testing.T) {
	provider := EnvGitAuthProvider{Lookup: func(key string) (string, bool) {
		switch key {
		case "GITHUB_TOKEN":
			return "github-secret", true
		case "GIT_TOKEN":
			return "git-secret", true
		default:
			return "", false
		}
	}}

	auth := provider.Auth()
	basic, ok := auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("Auth() = %T, want *http.BasicAuth", auth)
	}
	if got, want := basic.Username, "token"; got != want {
		t.Fatalf("Username = %q, want %q", got, want)
	}
	if got, want := basic.Password, "github-secret"; got != want {
		t.Fatalf("Password = %q, want %q", got, want)
	}
}

func TestEnvGitAuthProviderFallsBackToGitToken(t *testing.T) {
	provider := EnvGitAuthProvider{Lookup: func(key string) (string, bool) {
		if key == "GIT_TOKEN" {
			return "git-secret", true
		}
		return "", false
	}}

	auth := provider.Auth()
	basic, ok := auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("Auth() = %T, want *http.BasicAuth", auth)
	}
	if got, want := basic.Password, "git-secret"; got != want {
		t.Fatalf("Password = %q, want %q", got, want)
	}
}

func TestGitFetcherSkipsPublicPackages(t *testing.T) {
	fetcher := GitFetcher{GOPRIVATE: "github.com/private/*", Cloner: &fakeGitCloner{}}

	_, err := fetcher.Fetch(context.Background(), "github.com/public/mod", Options{})
	if err != ErrSourceNotApplicable {
		t.Fatalf("Fetch() error = %v, want ErrSourceNotApplicable", err)
	}
}

func TestGitFetcherUsesMetaDiscovery(t *testing.T) {
	discovery := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<meta name="go-import" content="private.example.com/root git https://private.example.com/root.git">`))
	}))
	t.Cleanup(discovery.Close)

	repo := afero.NewMemMapFs()
	writeTestFile(t, repo, "/pkg/pkg.go", []byte("package pkg\n"))
	cloner := &fakeGitCloner{fs: repo}
	fetcher := GitFetcher{
		GOPRIVATE:    "private.example.com/*",
		HTTPClient:   discovery.Client(),
		DiscoveryURL: func(string) string { return discovery.URL + "?go-get=1" },
		Cloner:       cloner,
	}

	source, err := fetcher.Fetch(context.Background(), "private.example.com/root/pkg", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got, want := cloner.last.RepoURL, "https://private.example.com/root.git"; got != want {
		t.Fatalf("RepoURL = %q, want %q", got, want)
	}
	if got, want := source.ImportPath, "private.example.com/root/pkg"; got != want {
		t.Fatalf("ImportPath = %q, want %q", got, want)
	}
}

func TestGitFetcherFallsBackToHostConvention(t *testing.T) {
	repo := afero.NewMemMapFs()
	writeTestFile(t, repo, "/pkg/pkg.go", []byte("package pkg\n"))
	cloner := &fakeGitCloner{fs: repo}
	fetcher := GitFetcher{
		GOPRIVATE: "github.com/private/*",
		Cloner:    cloner,
	}

	_, err := fetcher.Fetch(context.Background(), "github.com/private/mod/pkg", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got, want := cloner.last.RepoURL, "https://github.com/private/mod.git"; got != want {
		t.Fatalf("RepoURL = %q, want %q", got, want)
	}
}

func TestGitFetcherUsesTagForConcreteVersionAndWritesCache(t *testing.T) {
	repo := afero.NewMemMapFs()
	writeTestFile(t, repo, "/pkg/pkg.go", []byte("package pkg\n"))
	cacheFS := afero.NewMemMapFs()
	cloner := &fakeGitCloner{fs: repo}
	fetcher := GitFetcher{
		GOPRIVATE: "github.com/private/*",
		Cloner:    cloner,
		CacheFS:   cacheFS,
		CacheDir:  "/mod",
	}

	_, err := fetcher.Fetch(context.Background(), "github.com/private/mod/pkg", Options{Version: "v1.0.0"})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got, want := cloner.last.ReferenceName, plumbing.NewTagReferenceName("v1.0.0"); got != want {
		t.Fatalf("ReferenceName = %q, want %q", got, want)
	}

	cacheFetcher := ModCacheFetcher{FS: cacheFS, CacheDir: "/mod"}
	source, err := cacheFetcher.Fetch(context.Background(), "github.com/private/mod/pkg", Options{})
	if err != nil {
		t.Fatalf("cache Fetch() error = %v", err)
	}
	if got, want := len(source.Files), 1; got != want {
		t.Fatalf("len(Files) = %d, want %d", got, want)
	}
}

type fakeGitCloner struct {
	fs   afero.Fs
	last GitCloneRequest
}

func (c *fakeGitCloner) Clone(_ context.Context, req GitCloneRequest) (afero.Fs, error) {
	c.last = req
	if c.fs == nil {
		c.fs = afero.NewMemMapFs()
	}
	return c.fs, nil
}
