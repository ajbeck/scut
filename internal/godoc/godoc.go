package godoc

import (
	"context"
	"errors"
	"fmt"
	"go/doc"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

var ErrPackageRequired = errors.New("package is required")

// Client orchestrates source resolution, parsing, and formatting.
type Client struct {
	Resolver     Resolver
	PackageIndex PackageIndex
	Current      CurrentPackage
}

func (c Client) Doc(ctx context.Context, opts Options) (string, error) {
	resolved, err := LookupResolver{
		Resolver:     c.Resolver,
		PackageIndex: c.PackageIndex,
		Current:      c.Current,
	}.Resolve(ctx, opts)
	if err != nil {
		return "", err
	}
	source := resolved.Source

	mode := doc.AllDecls
	if opts.Src {
		mode |= doc.PreserveAST
	}

	parsed, err := ParsePackage(source.ImportPath, source.Files, mode)
	if err != nil {
		return "", err
	}
	return FormatLookup(parsed, resolved.Lookup, opts)
}

// NewDefaultClient assembles production source fetchers.
func NewDefaultClient(fs afero.Fs) (*Client, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolving working directory: %w", err)
	}
	moduleDir, modulePath, deps, replacements := readCurrentModule(fs, wd)
	cacheDir := defaultModuleCacheDir()

	fetchers := []SourceFetcher{}
	if moduleDir != "" && modulePath != "" {
		fetchers = append(fetchers, LocalSourceFetcher{
			FS:         fs,
			ModuleDir:  moduleDir,
			ModulePath: modulePath,
		})
	}
	fetchers = append(fetchers,
		StdlibSourceFetcher{FS: fs, GOROOT: runtime.GOROOT()},
		ReplaceSourceFetcher{FS: fs, Replacements: replacements},
		ModCacheFetcher{FS: fs, CacheDir: cacheDir, Deps: deps},
		GitFetcher{
			GOPRIVATE:    os.Getenv("GOPRIVATE"),
			DiscoveryURL: defaultDiscoveryURL,
			CacheFS:      fs,
			CacheDir:     cacheDir,
		},
		ProxyFetcher{
			ProxyURL:     defaultProxyURL(),
			DiscoveryURL: defaultDiscoveryURL,
			CacheFS:      fs,
			CacheDir:     cacheDir,
		},
	)

	return &Client{
		Resolver: Resolver{Fetchers: fetchers},
		PackageIndex: LocalPackageIndex{
			FS:         fs,
			ModuleDir:  moduleDir,
			ModulePath: modulePath,
			GOROOT:     runtime.GOROOT(),
			ModCache:   cacheDir,
		},
		Current: CurrentPackage{
			WorkDir:    wd,
			ModuleDir:  moduleDir,
			ModulePath: modulePath,
		},
	}, nil
}

func readCurrentModule(fs afero.Fs, start string) (string, string, map[string]module.Version, map[string]string) {
	dir := filepath.Clean(start)
	for {
		modPath := filepath.Join(dir, "go.mod")
		data, err := afero.ReadFile(fs, modPath)
		if err == nil {
			file, err := modfile.Parse(modPath, data, nil)
			if err != nil || file.Module == nil {
				return "", "", nil, nil
			}
			deps := make(map[string]module.Version, len(file.Require))
			for _, req := range file.Require {
				deps[req.Mod.Path] = req.Mod
			}
			replacements := localReplacements(dir, file)
			return dir, file.Module.Mod.Path, deps, replacements
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", nil, nil
		}
		dir = parent
	}
}

func localReplacements(moduleDir string, file *modfile.File) map[string]string {
	replacements := make(map[string]string)
	for _, replace := range file.Replace {
		if replace.New.Version != "" || !isLocalPackageArg(replace.New.Path) {
			continue
		}
		dir := replace.New.Path
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(moduleDir, dir)
		}
		replacements[replace.Old.Path] = filepath.Clean(dir)
	}
	return replacements
}

func defaultModuleCacheDir() string {
	if cacheDir := os.Getenv("GOMODCACHE"); cacheDir != "" {
		return cacheDir
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		return filepath.Join(strings.Split(gopath, string(os.PathListSeparator))[0], "pkg", "mod")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "go", "pkg", "mod")
	}
	return ""
}

func defaultProxyURL() string {
	proxy := os.Getenv("GOPROXY")
	if proxy == "" {
		return "https://proxy.golang.org"
	}
	proxy = strings.FieldsFunc(proxy, func(r rune) bool { return r == ',' || r == '|' })[0]
	if proxy == "" || proxy == "direct" || proxy == "off" {
		return "https://proxy.golang.org"
	}
	return proxy
}

func defaultDiscoveryURL(importPath string) string {
	return "https://" + importPath + "?go-get=1"
}
