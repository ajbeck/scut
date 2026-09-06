package godoc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/spf13/afero"
	"golang.org/x/mod/module"
)

func TestEnvGitAuthProviderPrefersGHToken(t *testing.T) {
	cli := &fakeGitHubTokenProvider{token: "cli-secret"}
	provider := EnvGitAuthProvider{Lookup: func(key string) (string, bool) {
		switch key {
		case "GH_TOKEN":
			return "gh-secret", true
		case "GITHUB_TOKEN":
			return "github-secret", true
		case "GIT_TOKEN":
			return "git-secret", true
		default:
			return "", false
		}
	}, GitHubTokenProvider: cli}

	auth := provider.Auth(context.Background(), "https://github.com/private/mod.git")
	basic, ok := auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("Auth() = %T, want *http.BasicAuth", auth)
	}
	if got, want := basic.Username, "token"; got != want {
		t.Fatalf("Username = %q, want %q", got, want)
	}
	if got, want := basic.Password, "gh-secret"; got != want {
		t.Fatalf("Password = %q, want %q", got, want)
	}
	if got, want := cli.calls, 0; got != want {
		t.Fatalf("gh token calls = %d, want %d", got, want)
	}
}

func TestEnvGitAuthProviderFallsBackToGitHubToken(t *testing.T) {
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

	auth := provider.Auth(context.Background(), "https://github.com/private/mod.git")
	basic, ok := auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("Auth() = %T, want *http.BasicAuth", auth)
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

	auth := provider.Auth(context.Background(), "https://github.com/private/mod.git")
	basic, ok := auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("Auth() = %T, want *http.BasicAuth", auth)
	}
	if got, want := basic.Password, "git-secret"; got != want {
		t.Fatalf("Password = %q, want %q", got, want)
	}
}

func TestEnvGitAuthProviderUsesGitHubCLIWhenNoEnvironmentToken(t *testing.T) {
	cli := &fakeGitHubTokenProvider{token: "cli-secret"}
	provider := EnvGitAuthProvider{
		Lookup:              func(string) (string, bool) { return "", false },
		GitHubTokenProvider: cli,
	}

	auth := provider.Auth(context.Background(), "https://github.example.com/private/mod.git")
	basic, ok := auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("Auth() = %T, want *http.BasicAuth", auth)
	}
	if got, want := basic.Password, "cli-secret"; got != want {
		t.Fatalf("Password = %q, want %q", got, want)
	}
	if got, want := cli.host, "github.example.com"; got != want {
		t.Fatalf("hostname = %q, want %q", got, want)
	}
	if !cli.hadDeadline {
		t.Fatal("gh token context has no deadline")
	}
}

func TestEnvGitAuthProviderSwallowsGitHubCLIErrorAndSkipsSSH(t *testing.T) {
	cli := &fakeGitHubTokenProvider{err: errors.New("gh unavailable")}
	provider := EnvGitAuthProvider{
		Lookup:              func(string) (string, bool) { return "", false },
		GitHubTokenProvider: cli,
		GitHubTokenTimeout:  time.Second,
	}

	if auth := provider.Auth(context.Background(), "https://github.com/private/mod.git"); auth != nil {
		t.Fatalf("Auth() = %T, want nil", auth)
	}
	if auth := provider.Auth(context.Background(), "git@github.com:private/mod.git"); auth != nil {
		t.Fatalf("Auth() for SSH = %T, want nil", auth)
	}
	if got, want := cli.calls, 1; got != want {
		t.Fatalf("gh token calls = %d, want %d", got, want)
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
		AuthProvider: nilGitAuthProvider{},
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
		GOPRIVATE:    "github.com/private/*",
		AuthProvider: nilGitAuthProvider{},
		Cloner:       cloner,
	}

	_, err := fetcher.Fetch(context.Background(), "github.com/private/mod/pkg", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got, want := cloner.last.RepoURL, "https://github.com/private/mod.git"; got != want {
		t.Fatalf("RepoURL = %q, want %q", got, want)
	}
}

func TestGitFetcherUsesTagForConcreteVersion(t *testing.T) {
	repo := afero.NewMemMapFs()
	writeTestFile(t, repo, "/pkg/pkg.go", []byte("package pkg\n"))
	writeTestFile(t, repo, "/internal/helper/helper.go", []byte("package helper\n"))
	store := FileArchiveStore{Root: t.TempDir()}
	cloner := &fakeGitCloner{
		fs:       repo,
		revision: "0123456789012345678901234567890123456789",
	}
	fetcher := GitFetcher{
		GOPRIVATE:    "github.com/private/*",
		AuthProvider: nilGitAuthProvider{},
		Cloner:       cloner,
		Store:        store,
	}

	_, err := fetcher.Fetch(context.Background(), "github.com/private/mod/pkg", Options{Version: "v1.0.0"})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got, want := cloner.last.ReferenceName, plumbing.NewTagReferenceName("v1.0.0"); got != want {
		t.Fatalf("ReferenceName = %q, want %q", got, want)
	}

	cached, err := (ArchiveFetcher{Store: store}).Fetch(
		context.Background(),
		"github.com/private/mod/internal/helper",
		Options{Version: "v1.0.0"},
	)
	if err != nil {
		t.Fatalf("cached Fetch() error = %v", err)
	}
	if got, want := len(cached.Files), 1; got != want {
		t.Fatalf("len(cached.Files) = %d, want %d", got, want)
	}
	archive, err := store.Get(context.Background(), module.Version{Path: "github.com/private/mod", Version: "v1.0.0"})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got, want := archive.Revision, cloner.revision; got != want {
		t.Fatalf("Revision = %q, want %q", got, want)
	}
}

func TestGitFetcherDoesNotCacheFloatingVersion(t *testing.T) {
	repo := afero.NewMemMapFs()
	writeTestFile(t, repo, "/pkg/pkg.go", []byte("package pkg\n"))
	cacheRoot := t.TempDir()
	fetcher := GitFetcher{
		GOPRIVATE:    "github.com/private/*",
		AuthProvider: nilGitAuthProvider{},
		Cloner: &fakeGitCloner{
			fs:       repo,
			revision: "0123456789012345678901234567890123456789",
		},
		Store: FileArchiveStore{Root: cacheRoot},
	}

	if _, err := fetcher.Fetch(context.Background(), "github.com/private/mod/pkg", Options{}); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("floating Git lookup created %d cache entries, want none", len(entries))
	}
}

func TestGitFetcherRetriesGitHubAuthenticationFailureWithSSHAgent(t *testing.T) {
	repo := afero.NewMemMapFs()
	writeTestFile(t, repo, "/pkg/pkg.go", []byte("package pkg\n"))
	cloner := &fakeGitCloner{fs: repo, errors: []error{transport.ErrAuthenticationRequired, nil}}
	ssh := &fakeSSHAuthProvider{auth: &githttp.BasicAuth{Username: "git", Password: "unused"}}
	fetcher := GitFetcher{
		GOPRIVATE:       "github.com/private/*",
		AuthProvider:    nilGitAuthProvider{},
		SSHAuthProvider: ssh,
		Cloner:          cloner,
	}

	_, err := fetcher.Fetch(context.Background(), "github.com/private/mod/pkg", Options{Version: "v1.0.0"})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got, want := len(cloner.requests), 2; got != want {
		t.Fatalf("clone calls = %d, want %d", got, want)
	}
	if got, want := cloner.requests[0].RepoURL, "https://github.com/private/mod.git"; got != want {
		t.Fatalf("HTTPS RepoURL = %q, want %q", got, want)
	}
	if got, want := cloner.requests[1].RepoURL, "git@github.com:private/mod.git"; got != want {
		t.Fatalf("SSH RepoURL = %q, want %q", got, want)
	}
	if got, want := cloner.requests[1].ReferenceName, plumbing.NewTagReferenceName("v1.0.0"); got != want {
		t.Fatalf("SSH ReferenceName = %q, want %q", got, want)
	}
	if got, want := ssh.calls, 1; got != want {
		t.Fatalf("SSH auth calls = %d, want %d", got, want)
	}
}

func TestGitFetcherDoesNotRetryNonAuthenticationFailure(t *testing.T) {
	cloner := &fakeGitCloner{errors: []error{errors.New("network unavailable")}}
	ssh := &fakeSSHAuthProvider{}
	fetcher := GitFetcher{
		GOPRIVATE:       "github.com/private/*",
		AuthProvider:    nilGitAuthProvider{},
		SSHAuthProvider: ssh,
		Cloner:          cloner,
	}

	_, err := fetcher.Fetch(context.Background(), "github.com/private/mod/pkg", Options{})
	if err == nil {
		t.Fatal("Fetch() error = nil, want clone error")
	}
	if got, want := len(cloner.requests), 1; got != want {
		t.Fatalf("clone calls = %d, want %d", got, want)
	}
	if got, want := ssh.calls, 0; got != want {
		t.Fatalf("SSH auth calls = %d, want %d", got, want)
	}
}

func TestGitFetcherPreservesHTTPSAuthErrorWhenSSHIsUnavailable(t *testing.T) {
	cloneErr := transport.ErrAuthorizationFailed
	cloner := &fakeGitCloner{errors: []error{cloneErr}}
	ssh := &fakeSSHAuthProvider{err: errors.New("no SSH agent")}
	fetcher := GitFetcher{
		GOPRIVATE:       "github.com/private/*",
		AuthProvider:    nilGitAuthProvider{},
		SSHAuthProvider: ssh,
		Cloner:          cloner,
	}

	_, err := fetcher.Fetch(context.Background(), "github.com/private/mod/pkg", Options{})
	if !errors.Is(err, cloneErr) {
		t.Fatalf("Fetch() error = %v, want original HTTPS authentication error", err)
	}
	if got, want := len(cloner.requests), 1; got != want {
		t.Fatalf("clone calls = %d, want %d", got, want)
	}
}

func TestGitFetcherPreservesHTTPSAuthErrorWhenSSHRetryFails(t *testing.T) {
	cloneErr := transport.ErrAuthenticationRequired
	cloner := &fakeGitCloner{fs: afero.NewMemMapFs(), errors: []error{cloneErr, errors.New("SSH denied")}}
	fetcher := GitFetcher{
		GOPRIVATE:       "github.com/private/*",
		AuthProvider:    nilGitAuthProvider{},
		SSHAuthProvider: &fakeSSHAuthProvider{auth: &githttp.BasicAuth{Username: "git", Password: "unused"}},
		Cloner:          cloner,
	}

	_, err := fetcher.Fetch(context.Background(), "github.com/private/mod/pkg", Options{})
	if !errors.Is(err, cloneErr) {
		t.Fatalf("Fetch() error = %v, want original HTTPS authentication error", err)
	}
	if got, want := len(cloner.requests), 2; got != want {
		t.Fatalf("clone calls = %d, want %d", got, want)
	}
}

type nilGitAuthProvider struct{}

func (nilGitAuthProvider) Auth(context.Context, string) transport.AuthMethod { return nil }

type fakeGitHubTokenProvider struct {
	token       string
	err         error
	host        string
	calls       int
	hadDeadline bool
}

func (p *fakeGitHubTokenProvider) Token(ctx context.Context, host string) (string, error) {
	p.calls++
	p.host = host
	_, p.hadDeadline = ctx.Deadline()
	return p.token, p.err
}

type fakeSSHAuthProvider struct {
	auth  transport.AuthMethod
	err   error
	calls int
}

func (p *fakeSSHAuthProvider) Auth() (transport.AuthMethod, error) {
	p.calls++
	return p.auth, p.err
}

type fakeGitCloner struct {
	fs       afero.Fs
	revision string
	last     GitCloneRequest
	requests []GitCloneRequest
	errors   []error
}

func (c *fakeGitCloner) Clone(_ context.Context, req GitCloneRequest) (GitCloneResult, error) {
	c.last = req
	c.requests = append(c.requests, req)
	if len(c.errors) > 0 {
		err := c.errors[0]
		c.errors = c.errors[1:]
		if err != nil {
			return GitCloneResult{}, err
		}
	}
	if c.fs == nil {
		c.fs = afero.NewMemMapFs()
	}
	return GitCloneResult{FS: c.fs, Revision: c.revision}, nil
}
