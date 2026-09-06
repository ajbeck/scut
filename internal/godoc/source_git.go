package godoc

import (
	"context"
	"crypto/tls"
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
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// GitCloneRequest describes one in-memory Git clone.
type GitCloneRequest struct {
	RepoURL         string
	Auth            transport.AuthMethod
	ReferenceName   plumbing.ReferenceName
	Revision        string
	InsecureSkipTLS bool
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
	GOAUTH              *GoAuthenticator
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
	if auth := p.GOAUTH.GitAuth(ctx, repoURL); auth != nil {
		return auth
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

// GitFetcher loads direct package source from Git repositories.
type GitFetcher struct {
	GOPRIVATE       string
	GONOPROXY       string
	GOINSECURE      string
	GOVCS           string
	RequireNoProxy  bool
	AllowPublic     bool
	HTTPClient      *http.Client
	DiscoveryURL    discoveryFunc
	AuthProvider    GitAuthProvider
	SSHAuthProvider sshAuthProvider
	Cloner          GitCloner
	Store           ArchiveStore
	Selector        ModuleSelector
	Authenticator   *GoAuthenticator
}

func (f GitFetcher) Fetch(ctx context.Context, pkg string, opts Options) (PackageSource, error) {
	if !f.AllowPublic && !f.isPrivate(pkg) {
		return PackageSource{}, ErrSourceNotApplicable
	}

	logical, sourceModule, query, selected := f.selectModule(ctx, pkg, opts)
	resolved, err := f.resolveRepository(ctx, sourceModule.Path, selected)
	if err != nil {
		return PackageSource{}, err
	}
	if !selected {
		sourceModule = module.Version{Path: resolved.modulePath, Version: query}
		logical = sourceModule
	}
	if f.RequireNoProxy && !matchesModulePattern(f.GONOPROXY, resolved.modulePath) {
		return PackageSource{}, errUseProxy
	}
	policy := ModuleDownloadPolicy{GOPRIVATE: f.GOPRIVATE, GOINSECURE: f.GOINSECURE, GOVCS: f.GOVCS}
	if err := checkGitAllowed(policy, resolved.modulePath); err != nil {
		return PackageSource{}, err
	}
	insecure := matchesModulePattern(f.GOINSECURE, resolved.modulePath)
	if err := checkRepositoryTransport(resolved.repoURL, insecure); err != nil {
		return PackageSource{}, err
	}
	layout, err := resolveGitModuleLayout(resolved)
	if err != nil {
		return PackageSource{}, err
	}

	req := GitCloneRequest{
		RepoURL:         resolved.repoURL,
		Auth:            f.authProvider().Auth(ctx, resolved.repoURL),
		InsecureSkipTLS: insecure,
	}
	concreteVersion := query != "" && query != "latest"
	if concreteVersion {
		if module.IsPseudoVersion(query) {
			req.Revision, err = module.PseudoVersionRev(query)
			if err != nil {
				return PackageSource{}, err
			}
		} else {
			tagVersion := strings.TrimSuffix(query, "+incompatible")
			req.ReferenceName = plumbing.NewTagReferenceName(layout.tagPrefix + tagVersion)
		}
	}

	clone, err := f.cloner().Clone(ctx, req)
	if err != nil && isAuthenticationFailure(err) {
		if retry, ok := f.retryWithGitHubSSH(ctx, req); ok {
			clone = retry
			err = nil
		}
	}
	if err != nil {
		if errors.Is(err, transport.ErrRepositoryNotFound) || errors.Is(err, plumbing.ErrReferenceNotFound) {
			return PackageSource{}, fmt.Errorf("cloning %s: %w", pkg, remoteNotFoundError(err.Error()))
		}
		return PackageSource{}, fmt.Errorf("cloning %s: %w", pkg, err)
	}
	moduleRoot, err := findGitModuleRoot(clone.FS, resolved, layout)
	if err != nil {
		return PackageSource{}, err
	}
	packageDir := path.Join(moduleRoot, packageSubdir(pkg, logical.Path))
	files, err := copyPackageFilesToMem(clone.FS, packageDir)
	if err != nil {
		return PackageSource{}, err
	}
	if concreteVersion && module.CanonicalVersion(sourceModule.Version) == sourceModule.Version && clone.Revision != "" && f.Store != nil {
		if archive, archiveErr := moduleArchiveFromFSRoot(sourceModule, clone.FS, moduleRoot, clone.Revision); archiveErr == nil {
			_ = f.Store.Put(ctx, archive)
		}
	}

	return PackageSource{
		ImportPath: pkg,
		Dir:        path.Join("/", "git", pkg),
		Files:      files,
		Module:     logical,
		Version:    logical.Version,
	}, nil
}

func (f GitFetcher) selectModule(ctx context.Context, pkg string, opts Options) (logical, source module.Version, query string, selected bool) {
	if f.Selector != nil {
		if selection, ok := f.Selector.Select(ctx, pkg, opts); ok && selection.Dir == "" && selection.Source.Path != "" {
			return selection.Module, selection.Source, selection.Source.Version, true
		}
	}
	return module.Version{}, module.Version{Path: pkg, Version: opts.Version}, opts.Version, false
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

func checkRepositoryTransport(repoURL string, insecure bool) error {
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return err
	}
	switch parsed.Scheme {
	case "https", "ssh", "git+ssh":
		return nil
	case "http", "git":
		if insecure {
			return nil
		}
		return fmt.Errorf("refusing insecure direct repository transport %s without matching GOINSECURE", parsed.Scheme)
	case "":
		if strings.Contains(repoURL, "@") {
			return nil
		}
	}
	return fmt.Errorf("unsupported direct repository transport %q", parsed.Scheme)
}

func (f GitFetcher) isPrivate(pkg string) bool {
	return f.GOPRIVATE != "" && module.MatchPrefixPatterns(f.GOPRIVATE, pkg)
}

type resolvedGitRepository struct {
	modulePath string
	codeRoot   string
	repoURL    string
	subdir     string
}

func (f GitFetcher) resolveRepository(ctx context.Context, moduleOrPackage string, exactModule bool) (resolvedGitRepository, error) {
	parts := strings.Split(moduleOrPackage, "/")
	if len(parts) >= 3 && hasStaticGitRoot(parts[0]) {
		codeRoot := strings.Join(parts[:3], "/")
		modulePath := codeRoot
		if exactModule {
			modulePath = moduleOrPackage
		}
		return resolvedGitRepository{modulePath: modulePath, codeRoot: codeRoot, repoURL: "https://" + codeRoot + ".git"}, nil
	}

	meta, err := discoverGoImport(ctx, f.httpClient(moduleOrPackage), moduleOrPackage, f.DiscoveryURL)
	if err != nil && f.DiscoveryURL == nil && matchesModulePattern(f.GOINSECURE, moduleOrPackage) {
		meta, err = discoverGoImport(ctx, f.httpClient(moduleOrPackage), moduleOrPackage, func(importPath string) string {
			return "http://" + importPath + "?go-get=1"
		})
	}
	if err == nil && meta.VCS == "git" {
		modulePath := meta.Prefix
		if exactModule {
			modulePath = moduleOrPackage
		}
		return resolvedGitRepository{
			modulePath: modulePath,
			codeRoot:   meta.Prefix,
			repoURL:    meta.Repo,
			subdir:     meta.SubDir,
		}, nil
	}

	if len(parts) < 3 {
		return resolvedGitRepository{}, ErrSourceNotApplicable
	}
	codeRoot := strings.Join(parts[:3], "/")
	modulePath := codeRoot
	if exactModule {
		modulePath = moduleOrPackage
	}
	return resolvedGitRepository{
		modulePath: modulePath,
		codeRoot:   codeRoot,
		repoURL:    "https://" + codeRoot + ".git",
	}, nil
}

func hasStaticGitRoot(host string) bool {
	return host == "github.com" || host == "bitbucket.org"
}

type gitModuleLayout struct {
	codeDir   string
	pathMajor string
	tagPrefix string
}

func resolveGitModuleLayout(repo resolvedGitRepository) (gitModuleLayout, error) {
	pathPrefix, pathMajor, ok := module.SplitPathVersion(repo.modulePath)
	if !ok {
		return gitModuleLayout{}, fmt.Errorf("invalid module path %q", repo.modulePath)
	}
	codeDir := ""
	if repo.codeRoot != repo.modulePath {
		if pathPrefix != repo.codeRoot && !strings.HasPrefix(pathPrefix, repo.codeRoot+"/") {
			return gitModuleLayout{}, fmt.Errorf("repository rooted at %s cannot contain module %s", repo.codeRoot, repo.modulePath)
		}
		codeDir = strings.Trim(pathPrefix[len(repo.codeRoot):], "/")
	}
	if repo.subdir != "" {
		codeDir = path.Join(codeDir, repo.subdir)
	}
	tagPrefix := ""
	if codeDir != "" {
		tagPrefix = codeDir + "/"
	}
	return gitModuleLayout{codeDir: codeDir, pathMajor: pathMajor, tagPrefix: tagPrefix}, nil
}

func findGitModuleRoot(fs afero.Fs, repo resolvedGitRepository, layout gitModuleLayout) (string, error) {
	candidates := []string{layout.codeDir}
	if layout.pathMajor != "" && repo.codeRoot != repo.modulePath && strings.HasPrefix(layout.pathMajor, "/") {
		candidates = append(candidates, path.Join(layout.codeDir, strings.TrimPrefix(layout.pathMajor, "/")))
	}
	for _, candidate := range candidates {
		name := path.Join("/", candidate, "go.mod")
		data, err := afero.ReadFile(fs, name)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", err
		}
		if modfile.ModulePath(data) == repo.modulePath {
			return candidate, nil
		}
	}
	if layout.codeDir == "" && layout.pathMajor == "" {
		return "", nil
	}
	return "", fmt.Errorf("module %s not found in repository %s", repo.modulePath, repo.repoURL)
}

func (f GitFetcher) httpClient(modulePath string) httpDoer {
	if f.HTTPClient != nil {
		return f.Authenticator.Client(f.HTTPClient)
	}
	if !matchesModulePattern(f.GOINSECURE, modulePath) {
		return f.Authenticator.Client(http.DefaultClient)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 -- explicitly requested by GOINSECURE.
	return f.Authenticator.Client(&http.Client{Transport: transport})
}

func (f GitFetcher) authProvider() GitAuthProvider {
	if f.AuthProvider != nil {
		return f.AuthProvider
	}
	return EnvGitAuthProvider{GOAUTH: f.Authenticator}
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
		URL:             req.RepoURL,
		Auth:            req.Auth,
		ReferenceName:   req.ReferenceName,
		InsecureSkipTLS: req.InsecureSkipTLS,
	}
	if req.ReferenceName != "" && req.Revision == "" {
		options.SingleBranch = true
	}
	repo, err := git.CloneContext(ctx, memory.NewStorage(), worktree, options)
	if err != nil {
		return GitCloneResult{}, err
	}
	if req.Revision != "" {
		worktreeView, err := repo.Worktree()
		if err != nil {
			return GitCloneResult{}, err
		}
		if err := worktreeView.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(req.Revision), Force: true}); err != nil {
			return GitCloneResult{}, err
		}
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
