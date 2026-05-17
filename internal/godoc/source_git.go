package godoc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
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

// GitCloner clones a repository and returns an afero filesystem rooted at it.
type GitCloner interface {
	Clone(context.Context, GitCloneRequest) (afero.Fs, error)
}

type GitAuthProvider interface {
	Auth() transport.AuthMethod
}

// EnvGitAuthProvider reads Git HTTPS tokens from the environment.
type EnvGitAuthProvider struct {
	Lookup func(string) (string, bool)
}

func (p EnvGitAuthProvider) Auth() transport.AuthMethod {
	lookup := p.Lookup
	if lookup == nil {
		lookup = os.LookupEnv
	}
	for _, key := range []string{"GITHUB_TOKEN", "GIT_TOKEN"} {
		if token, ok := lookup(key); ok && token != "" {
			return &githttp.BasicAuth{Username: "token", Password: token}
		}
	}
	return nil
}

// GitFetcher loads private package source from Git repositories.
type GitFetcher struct {
	GOPRIVATE    string
	HTTPClient   *http.Client
	DiscoveryURL discoveryFunc
	AuthProvider GitAuthProvider
	Cloner       GitCloner
	CacheFS      afero.Fs
	CacheDir     string
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
		Auth:    f.authProvider().Auth(),
	}
	concreteVersion := opts.Version != "" && opts.Version != "latest"
	if concreteVersion {
		req.ReferenceName = plumbing.NewTagReferenceName(opts.Version)
	}

	repoFS, err := f.cloner().Clone(ctx, req)
	if err != nil {
		return PackageSource{}, fmt.Errorf("cloning %s: %w", pkg, err)
	}
	files, err := copyPackageFilesToMem(repoFS, packageSubdir(pkg, resolved.modulePath))
	if err != nil {
		return PackageSource{}, err
	}

	if concreteVersion && f.CacheFS != nil && f.CacheDir != "" {
		_ = WriteCache(f.CacheFS, f.CacheDir, ResolvedModule{
			Path:        resolved.modulePath,
			Version:     opts.Version,
			PackagePath: pkg,
		}, files)
	}

	return PackageSource{
		ImportPath: pkg,
		Dir:        path.Join("/", "git", pkg),
		Files:      files,
		Module:     module.Version{Path: resolved.modulePath, Version: opts.Version},
		Version:    opts.Version,
	}, nil
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

func (f GitFetcher) cloner() GitCloner {
	if f.Cloner != nil {
		return f.Cloner
	}
	return GoGitCloner{}
}

// GoGitCloner clones with go-git into memory and exposes the worktree as afero.
type GoGitCloner struct{}

func (GoGitCloner) Clone(ctx context.Context, req GitCloneRequest) (afero.Fs, error) {
	worktree := memfs.New()
	options := &git.CloneOptions{
		URL:           req.RepoURL,
		Auth:          req.Auth,
		ReferenceName: req.ReferenceName,
	}
	if req.ReferenceName != "" {
		options.SingleBranch = true
	}
	if _, err := git.CloneContext(ctx, memory.NewStorage(), worktree, options); err != nil {
		return nil, err
	}
	fs := afero.NewMemMapFs()
	if err := copyBillyToAfero(worktree, fs, "."); err != nil {
		return nil, err
	}
	return fs, nil
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
