package godoc

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/mod/module"
)

// ProxyFetcher loads public module source from a single Go module proxy.
type ProxyFetcher struct {
	Client       *http.Client
	ProxyURL     string
	DiscoveryURL discoveryFunc
	Store        ArchiveStore
	Selector     ModuleSelector
}

var versionPattern = regexp.MustCompile(`"Version"\s*:\s*"([^"]+)"`)

var errProxyMiss = errors.New("module proxy miss")

func proxyURLsFromEnv(value string) []string {
	if value == "" {
		return []string{"https://proxy.golang.org"}
	}
	var urls []string
	for _, entry := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '|' }) {
		entry = strings.TrimSpace(entry)
		switch entry {
		case "", "direct":
			continue
		case "off":
			return urls
		default:
			urls = append(urls, entry)
		}
	}
	return urls
}

func (f ProxyFetcher) Fetch(ctx context.Context, pkg string, opts Options) (PackageSource, error) {
	if f.ProxyURL == "" {
		return PackageSource{}, ErrSourceNotApplicable
	}
	client := f.client()
	if f.Selector != nil {
		if selected, ok := f.Selector.Select(ctx, pkg, opts); ok && selected.Dir == "" && selected.Source.Version != "" {
			return f.fetchSelected(ctx, client, pkg, selected)
		}
	}
	for _, modPath := range f.moduleCandidates(ctx, client, pkg) {
		version, err := f.resolveVersion(ctx, client, modPath, opts.Version)
		if err != nil {
			if errors.Is(err, errProxyMiss) {
				continue
			}
			return PackageSource{}, err
		}
		archive, err := f.fetchArchive(ctx, client, modPath, version)
		if err != nil {
			if errors.Is(err, errProxyMiss) {
				continue
			}
			return PackageSource{}, err
		}
		f.cacheArchive(ctx, archive, opts.Version == "" || opts.Version == "latest")
		source, err := packageSourceFromArchive(archive, pkg, "proxy")
		if err != nil {
			if errors.Is(err, ErrNoGoFiles) {
				continue
			}
			return PackageSource{}, err
		}
		return source, nil
	}
	return PackageSource{}, ErrSourceNotApplicable
}

func (f ProxyFetcher) fetchSelected(ctx context.Context, client *http.Client, pkg string, selected ModuleSelection) (PackageSource, error) {
	archive, err := f.fetchArchive(ctx, client, selected.Source.Path, selected.Source.Version)
	if errors.Is(err, errProxyMiss) {
		return PackageSource{}, ErrSourceNotApplicable
	}
	if err != nil {
		return PackageSource{}, err
	}
	f.cacheArchive(ctx, archive, false)
	source, err := packageSourceFromSelectedArchive(archive, pkg, selected.Module, "proxy")
	if errors.Is(err, ErrNoGoFiles) {
		return PackageSource{}, &cachedPackageAbsentError{Module: selected.Module, Package: pkg}
	}
	return source, err
}

func (f ProxyFetcher) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return http.DefaultClient
}

func (f ProxyFetcher) moduleCandidates(ctx context.Context, client *http.Client, pkg string) []string {
	var candidates []string
	seen := map[string]bool{}
	add := func(modPath string) {
		if modPath == "" || seen[modPath] {
			return
		}
		if pkg != modPath && !strings.HasPrefix(pkg, modPath+"/") {
			return
		}
		seen[modPath] = true
		candidates = append(candidates, modPath)
	}

	if meta, err := discoverGoImport(ctx, client, pkg, f.DiscoveryURL); err == nil {
		add(meta.Prefix)
	}

	for _, candidate := range modulePathCandidates(pkg) {
		add(candidate)
	}
	return candidates
}

func (f ProxyFetcher) resolveVersion(ctx context.Context, client *http.Client, modPath, version string) (string, error) {
	if version == "" || version == "latest" {
		body, err := f.proxyGet(ctx, client, modPath, "@latest")
		if err != nil {
			return "", err
		}
		return parseProxyVersion(body)
	}
	body, err := f.proxyGet(ctx, client, modPath, "@v/"+version+".info")
	if err != nil {
		return "", err
	}
	resolved, err := parseProxyVersion(body)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func (f ProxyFetcher) fetchArchive(ctx context.Context, client *http.Client, modPath, version string) (ModuleArchive, error) {
	escapedVersion, err := module.EscapeVersion(version)
	if err != nil {
		return ModuleArchive{}, err
	}
	body, err := f.proxyGet(ctx, client, modPath, "@v/"+escapedVersion+".zip")
	if err != nil {
		return ModuleArchive{}, err
	}
	return ModuleArchive{
		Module: module.Version{Path: modPath, Version: version},
		Data:   body,
	}, nil
}

func (f ProxyFetcher) cacheArchive(ctx context.Context, archive ModuleArchive, latest bool) {
	if f.Store == nil || f.Store.Put(ctx, archive) != nil || !latest {
		return
	}
	_ = f.Store.SetLatest(ctx, archive.Module)
}

func (f ProxyFetcher) proxyGet(ctx context.Context, client *http.Client, modPath, suffix string) ([]byte, error) {
	escaped, err := module.EscapePath(modPath)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(f.ProxyURL, "/")+"/"+escaped+"/"+suffix, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
			return nil, errProxyMiss
		}
		return nil, fmt.Errorf("module proxy returned %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func parseProxyVersion(body []byte) (string, error) {
	match := versionPattern.FindSubmatch(body)
	if len(match) != 2 {
		return "", errors.New("module proxy response missing Version")
	}
	return string(match[1]), nil
}

func extractPackageZip(modPath, version, pkg string, body []byte) ([]SourceFile, error) {
	return extractPackageZipForModule(modPath, version, modPath, pkg, body)
}

func extractPackageZipForModule(modPath, version, logicalModulePath, pkg string, body []byte) ([]SourceFile, error) {
	reader := bytes.NewReader(body)
	zr, err := zip.NewReader(reader, int64(len(body)))
	if err != nil {
		return nil, err
	}

	mem := afero.NewMemMapFs()
	targetDir := "/pkg"
	root := modPath + "@" + version + "/"
	subdir := packageSubdir(pkg, logicalModulePath)
	if subdir != "" {
		subdir += "/"
	}
	prefix := root + subdir

	for _, file := range zr.File {
		if file.FileInfo().IsDir() || !strings.HasPrefix(file.Name, prefix) {
			continue
		}
		rel := strings.TrimPrefix(file.Name, prefix)
		if rel == "" || strings.Contains(rel, "/") || !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		data, err := readZipFile(file)
		if err != nil {
			return nil, err
		}
		if err := afero.WriteFile(mem, path.Join(targetDir, rel), data, 0644); err != nil {
			return nil, err
		}
	}
	return readGoFiles(mem, targetDir)
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
