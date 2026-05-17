package godoc

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/afero"
)

func TestLocalPackageIndexMatchesStdlibSuffix(t *testing.T) {
	fs := afero.NewMemMapFs()
	writePackage(t, fs, "/goroot/src/encoding/json", "json")
	index := LocalPackageIndex{FS: fs, GOROOT: "/goroot"}

	got, err := index.MatchSuffix("json")
	if err != nil {
		t.Fatalf("MatchSuffix() error = %v", err)
	}
	want := []IndexedPackage{{ImportPath: "encoding/json", Dir: filepath.Clean("/goroot/src/encoding/json")}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchSuffix(json) = %#v, want %#v", got, want)
	}

	got, err = index.MatchSuffix("json.Decoder")
	if err != nil {
		t.Fatalf("MatchSuffix(json.Decoder) error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("MatchSuffix(json.Decoder) = %#v, want empty", got)
	}
}

func TestLocalPackageIndexMatchOrder(t *testing.T) {
	fs := afero.NewMemMapFs()
	writePackage(t, fs, "/goroot/src/text/template", "template")
	writePackage(t, fs, "/goroot/src/html/template", "template")
	index := LocalPackageIndex{FS: fs, GOROOT: "/goroot"}

	got, err := index.MatchSuffix("template")
	if err != nil {
		t.Fatalf("MatchSuffix() error = %v", err)
	}
	gotPaths := importPaths(got)
	wantPaths := []string{"html/template", "text/template"}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("MatchSuffix(template) paths = %#v, want %#v", gotPaths, wantPaths)
	}
}

func TestLocalPackageIndexStopsAtFirstMatchingTier(t *testing.T) {
	fs := afero.NewMemMapFs()
	writePackage(t, fs, "/goroot/src/encoding/json", "json")
	writePackage(t, fs, "/gomod/cache/github.com/acme/json@v1.0.0", "json")
	index := LocalPackageIndex{
		FS:       fs,
		GOROOT:   "/goroot",
		ModCache: "/gomod/cache",
	}

	got, err := index.MatchSuffix("json")
	if err != nil {
		t.Fatalf("MatchSuffix() error = %v", err)
	}
	gotPaths := importPaths(got)
	wantPaths := []string{"encoding/json"}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("MatchSuffix(json) paths = %#v, want %#v", gotPaths, wantPaths)
	}
}

func TestLocalPackageIndexMatchesSuffixPath(t *testing.T) {
	fs := afero.NewMemMapFs()
	writePackage(t, fs, "/repo/nested/pkg", "pkg")
	index := LocalPackageIndex{
		FS:         fs,
		ModuleDir:  "/repo",
		ModulePath: "example.com/root",
	}

	got, err := index.MatchSuffix("nested/pkg")
	if err != nil {
		t.Fatalf("MatchSuffix() error = %v", err)
	}
	want := []IndexedPackage{{ImportPath: "example.com/root/nested/pkg", Dir: filepath.Clean("/repo/nested/pkg")}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchSuffix(nested/pkg) = %#v, want %#v", got, want)
	}
}

func TestLocalPackageIndexSkipsNonPackages(t *testing.T) {
	fs := afero.NewMemMapFs()
	writePackage(t, fs, "/goroot/src/encoding/json", "json")
	writePackage(t, fs, "/goroot/src/vendor/example.com/json", "json")
	writePackage(t, fs, "/goroot/src/testdata/json", "json")
	writeTestOnlyPackage(t, fs, "/goroot/src/onlytest/json", "json")
	if err := fs.MkdirAll("/goroot/src/empty/json", 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	index := LocalPackageIndex{FS: fs, GOROOT: "/goroot"}

	got, err := index.MatchSuffix("json")
	if err != nil {
		t.Fatalf("MatchSuffix() error = %v", err)
	}
	gotPaths := importPaths(got)
	wantPaths := []string{"encoding/json"}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("MatchSuffix(json) paths = %#v, want %#v", gotPaths, wantPaths)
	}
}

func TestLocalPackageIndexIncludesModuleCacheWithoutNetwork(t *testing.T) {
	fs := afero.NewMemMapFs()
	writePackage(t, fs, "/gomod/cache/github.com/acme/lib@v1.2.3/sub", "sub")
	index := LocalPackageIndex{FS: fs, ModCache: "/gomod/cache"}

	got, err := index.MatchSuffix("lib/sub")
	if err != nil {
		t.Fatalf("MatchSuffix() error = %v", err)
	}
	want := []IndexedPackage{{ImportPath: "github.com/acme/lib/sub", Dir: filepath.Clean("/gomod/cache/github.com/acme/lib@v1.2.3/sub")}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchSuffix(lib/sub) = %#v, want %#v", got, want)
	}
}

func TestNewDefaultClientUsesLocalPackageIndex(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	fs := afero.NewMemMapFs()
	if err := fs.MkdirAll(wd, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := afero.WriteFile(fs, filepath.Join(wd, "go.mod"), []byte("module example.com/root\n"), 0644); err != nil {
		t.Fatalf("WriteFile(go.mod) error = %v", err)
	}

	client, err := NewDefaultClient(fs)
	if err != nil {
		t.Fatalf("NewDefaultClient() error = %v", err)
	}
	index, ok := client.PackageIndex.(LocalPackageIndex)
	if !ok {
		t.Fatalf("PackageIndex = %T, want LocalPackageIndex", client.PackageIndex)
	}
	if index.ModuleDir != wd || index.ModulePath != "example.com/root" {
		t.Fatalf("PackageIndex module = %q %q, want %q %q", index.ModuleDir, index.ModulePath, wd, "example.com/root")
	}
	if index.GOROOT == "" {
		t.Fatal("PackageIndex GOROOT is empty")
	}
}

func writePackage(t *testing.T, fs afero.Fs, dir, packageName string) {
	t.Helper()
	if err := fs.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", dir, err)
	}
	src := []byte("package " + packageName + "\n")
	if err := afero.WriteFile(fs, filepath.Join(dir, packageName+".go"), src, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func writeTestOnlyPackage(t *testing.T, fs afero.Fs, dir, packageName string) {
	t.Helper()
	if err := fs.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", dir, err)
	}
	src := []byte("package " + packageName + "\n")
	if err := afero.WriteFile(fs, filepath.Join(dir, packageName+"_test.go"), src, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func importPaths(pkgs []IndexedPackage) []string {
	paths := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		paths = append(paths, pkg.ImportPath)
	}
	return paths
}
