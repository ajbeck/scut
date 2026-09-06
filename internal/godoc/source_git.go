package godoc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/spf13/afero"
	"golang.org/x/mod/module"
)

// GitCloneRequest describes one in-memory Git clone.
type GitCloneRequest struct {
	RepoURL       string
	Auth          transport.AuthMethod
	ReferenceName plumbing.ReferenceName
}

// GitCloneResult is an in-memory repository snapshot and the immutable commit
// selected by the clone.
type GitCloneResult struct {
	FS       afero.Fs
	Revision string
}

// GitCloner clones a repository and returns its files and resolved revision.
type GitCloner interface {
	Clone(context.Context, GitCloneRequest) (GitCloneResult, error)
}

type GitAuthProvider interface {
	Auth(context.Context, string) transport.AuthMethod
}

type gitHubTokenProvider interface {
	Token(context.Context, string) (string, error)
}

type gitHubCLITokenProvider struct{}

func (gitHubCLITokenProvider) Token(ctx context.Context, host string) (string, error) {
	output, err := exec.CommandContext(ctx, "gh", "auth", "token", "--hostname", host).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

type sshAuthProvider interface {
	Auth() (transport.AuthMethod, error)
}

type sshAgentAuthProvider struct{}

func (sshAgentAuthProvider) Auth() (transport.AuthMethod, error) {
	return gitssh.NewSSHAgentAuth("git")
}

// EnvGitAuthProvider reads HTTPS Git tokens from the environment or gh.
type EnvGitAuthProvider struct {
	Lookup              func(string) (string, bool)
	GitHubTokenProvider gitHubTokenProvider
	GitHubTokenTimeout  time.Duration
}

func (p EnvGitAuthProvider) Auth(ctx context.Context, repoURL string) transport.AuthMethod {
	host, ok := httpsRepositoryHost(repoURL)
	if !ok {
		return nil
	}
	lookup := p.Lookup
	if lookup == nil {
		lookup = os.LookupEnv
	}
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GIT_TOKEN"} {
		if token, ok := lookup(key); ok && token != "" {
			return &githttp.BasicAuth{Username: "token", Password: token}
		}
	}

	timeout := p.GitHubTokenTimeout
	if timeout <= 0 {
		timeout = time.Second
	}
	tokenCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	provider := p.GitHubTokenProvider
	if provider == nil {
		provider = gitHubCLITokenProvider{}
	}
	token, err := provider.Token(tokenCtx, host)
	if err == nil && token != "" {
		return &githttp.BasicAuth{Username: "token", Password: token}
	}
	return nil
}

func httpsRepositoryHost(repoURL string) (string, bool) {
	parsed, err := url.Parse(repoURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" {
		return "", false
	}
	return parsed.Hostname(), true
}

// GitFetcher loads private package source from Git repositories.
type GitFetcher struct {
	GOPRIVATE       string
	HTTPClient      *http.Client
	DiscoveryURL    discoveryFunc
	AuthProvider    GitAuthProvider
	SSHAuthProvider sshAuthProvider
	Cloner          GitCloner
	Store           ArchiveStore
}

func (f GitFetcher) Fetch(ctx context.Context, pkg string, opts Options) (PackageSource, error) {
	if !f.isPrivate(pkg) {
		return PackageSource{}, ErrSourceNotApplicable
	}

	resolved, err := f.resolveRepository(ctx, pkg)
	if err != nil {
		return PackageSource{}, err
	}

	req := GitCloneRequest{
		RepoURL: resolved.repoURL,
		Auth:    f.authProvider().Auth(ctx, resolved.repoURL),
	}
	concreteVersion := opts.Version != "" && opts.Version != "latest"
	if concreteVersion {
		req.ReferenceName = plumbing.NewTagReferenceName(opts.Version)
	}

	clone, err := f.cloner().Clone(ctx, req)
	if err != nil && isAuthenticationFailure(err) {
		if retry, ok := f.retryWithGitHubSSH(ctx, req); ok {
			clone = retry
			err = nil
		}
	}
	if err != nil {
		return PackageSource{}, fmt.Errorf("cloning %s: %w", pkg, err)
	}
	files, err := copyPackageFilesToMem(clone.FS, packageSubdir(pkg, resolved.modulePath))
	if err != nil {
		return PackageSource{}, err
	}
	mod := module.Version{Path: resolved.modulePath, Version: opts.Version}
	if concreteVersion && module.CanonicalVersion(mod.Version) == mod.Version && clone.Revision != "" && f.Store != nil {
		if archive, archiveErr := moduleArchiveFromFS(mod, clone.FS, clone.Revision); archiveErr == nil {
			_ = f.Store.Put(ctx, archive)
		}
	}

	return PackageSource{
		ImportPath: pkg,
		Dir:        path.Join("/", "git", pkg),
		Files:      files,
		Module:     mod,
		Version:    opts.Version,
	}, nil
}

func isAuthenticationFailure(err error) bool {
	return errors.Is(err, transport.ErrAuthenticationRequired) || errors.Is(err, transport.ErrAuthorizationFailed)
}

func (f GitFetcher) retryWithGitHubSSH(ctx context.Context, request GitCloneRequest) (GitCloneResult, bool) {
	sshURL, ok := githubSSHURL(request.RepoURL)
	if !ok {
		return GitCloneResult{}, false
	}
	auth, err := f.sshAuthProvider().Auth()
	if err != nil {
		return GitCloneResult{}, false
	}
	request.RepoURL = sshURL
	request.Auth = auth
	clone, err := f.cloner().Clone(ctx, request)
	if err != nil {
		return GitCloneResult{}, false
	}
	return clone, true
}

func githubSSHURL(repoURL string) (string, bool) {
	parsed, err := url.Parse(repoURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || !strings.EqualFold(parsed.Hostname(), "github.com") || parsed.Port() != "" {
		return "", false
	}
	repositoryPath := strings.TrimPrefix(parsed.EscapedPath(), "/")
	if repositoryPath == "" {
		return "", false
	}
	return "git@github.com:" + repositoryPath, true
}

func (f GitFetcher) isPrivate(pkg string) bool {
	return f.GOPRIVATE != "" && module.MatchPrefixPatterns(f.GOPRIVATE, pkg)
}

type resolvedGitRepository struct {
	modulePath string
	repoURL    string
}

func (f GitFetcher) resolveRepository(ctx context.Context, pkg string) (resolvedGitRepository, error) {
	if f.DiscoveryURL != nil {
		meta, err := discoverGoImport(ctx, f.httpClient(), pkg, f.DiscoveryURL)
		if err == nil && meta.VCS == "git" {
			return resolvedGitRepository{modulePath: meta.Prefix, repoURL: meta.Repo}, nil
		}
	}

	parts := strings.Split(pkg, "/")
	if len(parts) < 3 {
		return resolvedGitRepository{}, ErrSourceNotApplicable
	}
	modulePath := strings.Join(parts[:3], "/")
	return resolvedGitRepository{
		modulePath: modulePath,
		repoURL:    "https://" + modulePath + ".git",
	}, nil
}

func (f GitFetcher) httpClient() *http.Client {
	if f.HTTPClient != nil {
		return f.HTTPClient
	}
	return http.DefaultClient
}

func (f GitFetcher) authProvider() GitAuthProvider {
	if f.AuthProvider != nil {
		return f.AuthProvider
	}
	return EnvGitAuthProvider{}
}

func (f GitFetcher) sshAuthProvider() sshAuthProvider {
	if f.SSHAuthProvider != nil {
		return f.SSHAuthProvider
	}
	return sshAgentAuthProvider{}
}

func (f GitFetcher) cloner() GitCloner {
	if f.Cloner != nil {
		return f.Cloner
	}
	return GoGitCloner{}
}

// GoGitCloner clones with go-git into memory and exposes the worktree as afero.
type GoGitCloner struct{}

func (GoGitCloner) Clone(ctx context.Context, req GitCloneRequest) (GitCloneResult, error) {
	worktree := memfs.New()
	options := &git.CloneOptions{
		URL:           req.RepoURL,
		Auth:          req.Auth,
		ReferenceName: req.ReferenceName,
	}
	if req.ReferenceName != "" {
		options.SingleBranch = true
	}
	repo, err := git.CloneContext(ctx, memory.NewStorage(), worktree, options)
	if err != nil {
		return GitCloneResult{}, err
	}
	head, err := repo.Head()
	if err != nil {
		return GitCloneResult{}, err
	}
	fs := afero.NewMemMapFs()
	if err := copyBillyToAfero(worktree, fs, "."); err != nil {
		return GitCloneResult{}, err
	}
	return GitCloneResult{FS: fs, Revision: head.Hash().String()}, nil
}

func copyBillyToAfero(src billy.Filesystem, dst afero.Fs, dir string) error {
	entries, err := src.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := src.Join(dir, entry.Name())
		if entry.IsDir() {
			if err := copyBillyToAfero(src, dst, name); err != nil {
				return err
			}
			continue
		}
		file, err := src.Open(name)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		target := filepath.Clean("/" + filepath.ToSlash(name))
		if err := dst.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := afero.WriteFile(dst, target, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func copyPackageFilesToMem(repoFS afero.Fs, subdir string) ([]SourceFile, error) {
	sourceDir := "/"
	if subdir != "" {
		sourceDir = path.Join("/", subdir)
	}
	files, err := readGoFiles(repoFS, filepath.FromSlash(sourceDir))
	if err != nil {
		return nil, err
	}
	mem := afero.NewMemMapFs()
	for _, file := range files {
		if err := afero.WriteFile(mem, filepath.Join("/pkg", filepath.Base(file.Name)), file.Data, 0644); err != nil {
			return nil, err
		}
	}
	return readGoFiles(mem, "/pkg")
}
