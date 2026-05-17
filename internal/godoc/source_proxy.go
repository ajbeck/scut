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
	"path/filepath"
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
	CacheFS      afero.Fs
	CacheDir     string
}

var versionPattern = regexp.MustCompile(`"Version"\s*:\s*"([^"]+)"`)

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
	for _, modPath := range f.moduleCandidates(ctx, client, pkg) {
		version, err := f.resolveVersion(ctx, client, modPath, opts.Version)
		if err != nil {
			continue
		}
		source, err := f.fetchZip(ctx, client, modPath, version, pkg)
		if err != nil {
			continue
		}
		if f.CacheFS != nil && f.CacheDir != "" {
			_ = WriteCache(f.CacheFS, f.CacheDir, ResolvedModule{
				Path:        modPath,
				Version:     version,
				PackagePath: pkg,
			}, source.Files)
		}
		return source, nil
	}
	return PackageSource{}, ErrSourceNotApplicable
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

	parts := strings.Split(pkg, "/")
	minParts := 2
	if len(parts) >= 3 && isCommonGitHost(parts[0]) {
		minParts = 3
	}
	for i := len(parts); i >= minParts; i-- {
		add(strings.Join(parts[:i], "/"))
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

func (f ProxyFetcher) fetchZip(ctx context.Context, client *http.Client, modPath, version, pkg string) (PackageSource, error) {
	escapedVersion, err := module.EscapeVersion(version)
	if err != nil {
		return PackageSource{}, err
	}
	body, err := f.proxyGet(ctx, client, modPath, "@v/"+escapedVersion+".zip")
	if err != nil {
		return PackageSource{}, err
	}

	files, err := extractPackageZip(modPath, version, pkg, body)
	if err != nil {
		return PackageSource{}, err
	}
	return PackageSource{
		ImportPath: pkg,
		Dir:        filepath.Join("/", "proxy", filepath.FromSlash(pkg)),
		Files:      files,
		Module:     module.Version{Path: modPath, Version: version},
		Version:    version,
	}, nil
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
	reader := bytes.NewReader(body)
	zr, err := zip.NewReader(reader, int64(len(body)))
	if err != nil {
		return nil, err
	}

	mem := afero.NewMemMapFs()
	targetDir := "/pkg"
	root := modPath + "@" + version + "/"
	subdir := packageSubdir(pkg, modPath)
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
