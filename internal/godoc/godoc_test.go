package godoc

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestClientDocFetchesParsesAndFormats(t *testing.T) {
	fetcher := &fakeSourceFetcher{
		source: PackageSource{
			ImportPath: "example.com/widgets",
			Files: []SourceFile{{
				Name: "widgets.go",
				Data: []byte(`// Package widgets manages widgets.
package widgets

// New returns a widget.
func New() {}
`),
			}},
		},
	}
	client := Client{Resolver: Resolver{Fetchers: []SourceFetcher{fetcher}}}

	out, err := client.Doc(context.Background(), Options{Package: "example.com/widgets", Symbol: "New"})
	if err != nil {
		t.Fatalf("Doc() error = %v", err)
	}

	if !strings.Contains(out, "func New()") {
		t.Fatalf("output missing function signature:\n%s", out)
	}
	if !strings.Contains(out, "New returns a widget") {
		t.Fatalf("output missing function docs:\n%s", out)
	}
}

func TestClientDocDefaultsToCurrentPackage(t *testing.T) {
	fetcher := &fakeSourceFetcher{
		source: PackageSource{
			ImportPath: "example.com/current",
			Files: []SourceFile{{
				Name: "current.go",
				Data: []byte(`// Package current is the working package.
package current
`),
			}},
		},
	}
	client := Client{Resolver: Resolver{Fetchers: []SourceFetcher{fetcher}}}

	out, err := client.Doc(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Doc() error = %v", err)
	}
	if !strings.Contains(out, "Package current is the working package") {
		t.Fatalf("output missing current package docs:\n%s", out)
	}
}

func TestClientDocUsesUnexportedAndPreserveASTModes(t *testing.T) {
	fetcher := &fakeSourceFetcher{
		source: PackageSource{
			ImportPath: "example.com/widgets",
			Files: []SourceFile{{
				Name: "widgets.go",
				Data: []byte(`package widgets

// hidden is internal.
func hidden() {
	println("kept")
}
`),
			}},
		},
	}
	client := Client{Resolver: Resolver{Fetchers: []SourceFetcher{fetcher}}}

	out, err := client.Doc(context.Background(), Options{
		Package:    "example.com/widgets",
		Symbol:     "hidden",
		Unexported: true,
		Src:        true,
	})
	if err != nil {
		t.Fatalf("Doc() error = %v", err)
	}

	if !strings.Contains(out, `println("kept")`) {
		t.Fatalf("source body was not preserved:\n%s", out)
	}
}

func TestClientDocParsesAllDeclsForDefaultVisibilityFiltering(t *testing.T) {
	fetcher := &fakeSourceFetcher{
		source: PackageSource{
			ImportPath: "example.com/widgets",
			Files: []SourceFile{{
				Name: "widgets.go",
				Data: []byte(`package widgets

// Widget stores visible and hidden state.
type Widget struct {
	Name string
	hidden string
}
`),
			}},
		},
	}
	client := Client{Resolver: Resolver{Fetchers: []SourceFetcher{fetcher}}}

	out, err := client.Doc(context.Background(), Options{Args: []string{"example.com/widgets", "Widget"}})
	if err != nil {
		t.Fatalf("Doc() error = %v", err)
	}

	if !strings.Contains(out, "// Has unexported fields.") {
		t.Fatalf("output missing go doc unexported field marker:\n%s", out)
	}
	if strings.Contains(out, "hidden string") {
		t.Fatalf("output leaked unexported field:\n%s", out)
	}
}

func TestClientDoesNotCreatePartialModuleCache(t *testing.T) {
	const (
		modPath = "example.com/acme/tool"
		version = "v1.2.3"
		pkg     = modPath + "/sub"
	)
	proxy := newModuleProxyServer(t, modPath, version, map[string]string{
		"go.mod": "module " + modPath + "\n\ngo 1.26\n",
		"internal/helper/helper.go": `package helper

const Value = "complete"
`,
		"sub/sub.go": `// Package sub exercises module-cache safety.
package sub

import "example.com/acme/tool/internal/helper"

// Value comes from another package in this module.
const Value = helper.Value
`,
	})
	t.Cleanup(proxy.Close)

	modCacheRoot := t.TempDir()
	modCache := filepath.Join(modCacheRoot, "modcache")
	t.Cleanup(func() {
		_ = filepath.Walk(modCacheRoot, func(name string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return os.Chmod(name, 0o700)
			}
			return os.Chmod(name, 0o600)
		})
		_ = os.RemoveAll(modCacheRoot)
	})
	lookupDir := t.TempDir()
	t.Chdir(lookupDir)
	t.Setenv("GOMODCACHE", modCache)
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "build-cache"))
	t.Setenv("GOPATH", filepath.Join(t.TempDir(), "gopath"))
	t.Setenv("GOPROXY", proxy.URL)
	t.Setenv("GOPRIVATE", "")
	t.Setenv("GONOPROXY", "")
	t.Setenv("GONOSUMDB", "")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOENV", "off")
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOTOOLCHAIN", "local")

	fs := afero.NewOsFs()
	client := newClient(fs, lookupDir, FileArchiveStore{Root: filepath.Join(t.TempDir(), "scut-cache")})
	out, err := client.Doc(t.Context(), Options{Package: pkg, Version: version})
	if err != nil {
		t.Fatalf("Doc() error = %v", err)
	}
	if !strings.Contains(out, "Package sub exercises module-cache safety") {
		t.Fatalf("Doc() output missing package documentation:\n%s", out)
	}

	moduleDir, err := moduleCachePath(modCache, modPath, version)
	if err != nil {
		t.Fatalf("moduleCachePath() error = %v", err)
	}
	if _, err := os.Stat(moduleDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("documentation lookup created module-cache directory %q: %v", moduleDir, err)
	}

	consumerDir := t.TempDir()
	writeTestFile(t, fs, filepath.Join(consumerDir, "go.mod"), []byte(`module example.com/consumer

go 1.26

require example.com/acme/tool v1.2.3
`))
	cmd := exec.CommandContext(t.Context(), "go", "list", "-mod=mod", pkg)
	cmd.Dir = consumerDir
	output, err := cmd.Output()
	if err != nil {
		stderr := ""
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			stderr = string(exitErr.Stderr)
		}
		t.Fatalf("go list after documentation lookup failed: %v\n%s", err, stderr)
	}
	if got, want := strings.TrimSpace(string(output)), pkg; got != want {
		t.Fatalf("go list output = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(moduleDir, "internal", "helper", "helper.go")); err != nil {
		t.Fatalf("Go did not extract the complete module after documentation lookup: %v", err)
	}
}

func TestReadCurrentModuleCollectsLocalReplacements(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeTestFile(t, fs, "/workspace/app/go.mod", []byte(`module example.com/app

require example.com/lib v1.2.3
replace example.com/lib => ../lib
replace example.com/remote => example.com/fork v1.0.0
`))

	moduleDir, modulePath, deps, replacements := readCurrentModule(fs, "/workspace/app/pkg")
	if got, want := moduleDir, "/workspace/app"; got != want {
		t.Fatalf("moduleDir = %q, want %q", got, want)
	}
	if got, want := modulePath, "example.com/app"; got != want {
		t.Fatalf("modulePath = %q, want %q", got, want)
	}
	if got, want := deps["example.com/lib"].Version, "v1.2.3"; got != want {
		t.Fatalf("dependency version = %q, want %q", got, want)
	}
	if got, want := replacements["example.com/lib"], filepath.Join("/workspace", "lib"); got != want {
		t.Fatalf("replacement = %q, want %q", got, want)
	}
	if _, ok := replacements["example.com/remote"]; ok {
		t.Fatal("remote replacement was collected, want only local replacements")
	}
}

func TestResolverRouteOrderFallsThroughOnNotApplicable(t *testing.T) {
	first := &fakeSourceFetcher{name: "first", err: ErrSourceNotApplicable}
	second := &fakeSourceFetcher{name: "second", source: PackageSource{ImportPath: "example.com/pkg"}}
	resolver := Resolver{Fetchers: []SourceFetcher{first, second}}

	source, err := resolver.Fetch(context.Background(), "example.com/pkg", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if got, want := source.ImportPath, "example.com/pkg"; got != want {
		t.Fatalf("ImportPath = %q, want %q", got, want)
	}
	if !first.called || !second.called {
		t.Fatalf("expected both fetchers to be called, first=%v second=%v", first.called, second.called)
	}
}

func TestResolverStopsAtFirstHit(t *testing.T) {
	first := &fakeSourceFetcher{name: "local", source: PackageSource{ImportPath: "local"}}
	second := &fakeSourceFetcher{name: "stdlib", source: PackageSource{ImportPath: "stdlib"}}
	resolver := Resolver{Fetchers: []SourceFetcher{first, second}}

	source, err := resolver.Fetch(context.Background(), "encoding/json", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got, want := source.ImportPath, "local"; got != want {
		t.Fatalf("ImportPath = %q, want %q", got, want)
	}
	if second.called {
		t.Fatal("second fetcher was called after first hit")
	}
}

func TestResolverReturnsPackageNotFoundWhenEveryFetcherMisses(t *testing.T) {
	resolver := Resolver{Fetchers: []SourceFetcher{
		&fakeSourceFetcher{name: "local", err: ErrSourceNotApplicable},
		&fakeSourceFetcher{name: "stdlib", err: ErrSourceNotApplicable},
	}}

	_, err := resolver.Fetch(context.Background(), "example.com/missing", Options{})
	if err == nil {
		t.Fatal("Fetch() error = nil, want package not found")
	}
	if !errors.Is(err, ErrPackageNotFound) {
		t.Fatalf("Fetch() error = %v, want ErrPackageNotFound", err)
	}
	if strings.Contains(err.Error(), ErrSourceNotApplicable.Error()) {
		t.Fatalf("Fetch() error exposes source fetcher sentinel: %v", err)
	}
	if !strings.Contains(err.Error(), "example.com/missing") {
		t.Fatalf("Fetch() error = %v, want requested package", err)
	}
}

func TestResolverStopsAtConcreteFetcherError(t *testing.T) {
	boom := errors.New("network failed")
	first := &fakeSourceFetcher{name: "local", err: ErrSourceNotApplicable}
	second := &fakeSourceFetcher{name: "proxy", err: boom}
	third := &fakeSourceFetcher{name: "git", source: PackageSource{ImportPath: "example.com/pkg"}}
	resolver := Resolver{Fetchers: []SourceFetcher{first, second, third}}

	_, err := resolver.Fetch(context.Background(), "example.com/pkg", Options{})
	if !errors.Is(err, boom) {
		t.Fatalf("Fetch() error = %v, want concrete fetcher error", err)
	}
	if !first.called || !second.called {
		t.Fatalf("expected first and second fetchers to be called, first=%v second=%v", first.called, second.called)
	}
	if third.called {
		t.Fatal("third fetcher was called after concrete error")
	}
}

type fakeSourceFetcher struct {
	name   string
	source PackageSource
	err    error
	called bool
}

func (f *fakeSourceFetcher) Fetch(context.Context, string, Options) (PackageSource, error) {
	f.called = true
	if f.err != nil {
		return PackageSource{}, f.err
	}
	return f.source, nil
}
