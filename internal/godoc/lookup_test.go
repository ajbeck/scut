package godoc

import (
	"context"
	"errors"
	"go/build"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/mod/module"
)

func TestParseSymbolSpec(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    *SymbolLookup
		wantErr string
	}{
		{name: "empty", raw: ""},
		{name: "symbol", raw: "Marshal", want: &SymbolLookup{Name: "Marshal"}},
		{name: "member", raw: "Decoder.Decode", want: &SymbolLookup{Name: "Decoder", Member: new("Decode")}},
		{name: "too_many_periods", raw: "a.b.c", wantErr: "too many periods in symbol specification"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSymbolSpec(tt.raw)
			if tt.wantErr != "" {
				if err == nil || !contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseSymbolSpec(%q) error = %v, want containing %q", tt.raw, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSymbolSpec(%q) error = %v", tt.raw, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseSymbolSpec(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestLookupCandidatesOneArgumentFullPath(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []LookupCandidate
	}{
		{
			name: "function",
			args: []string{"encoding/json.Marshal"},
			want: []LookupCandidate{
				{Package: "encoding/json.Marshal", UserPath: "encoding/json.Marshal", Kind: LookupFullPackage},
				{Package: "encoding/json", UserPath: "encoding/json", Symbol: &SymbolLookup{Name: "Marshal"}, Kind: LookupFullPackage, ContinueOnSymbolMiss: true},
			},
		},
		{
			name: "member",
			args: []string{"encoding/json.Decoder.Decode"},
			want: []LookupCandidate{
				{Package: "encoding/json.Decoder.Decode", UserPath: "encoding/json.Decoder.Decode", Kind: LookupFullPackage},
				{Package: "encoding/json", UserPath: "encoding/json", Symbol: &SymbolLookup{Name: "Decoder", Member: new("Decode")}, Kind: LookupFullPackage, ContinueOnSymbolMiss: true},
			},
		},
		{
			name: "dotted_package_path",
			args: []string{"gopkg.in/yaml.v3.Node"},
			want: []LookupCandidate{
				{Package: "gopkg.in/yaml.v3.Node", UserPath: "gopkg.in/yaml.v3.Node", Kind: LookupFullPackage},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lookupCandidates(tt.args)
			if err != nil {
				t.Fatalf("lookupCandidates(%q) error = %v", tt.args, err)
			}
			assertCandidatePrefix(t, got, tt.want)
			if tt.name == "dotted_package_path" {
				assertCandidateContains(t, got, LookupCandidate{
					Package:              "gopkg.in/yaml.v3",
					UserPath:             "gopkg.in/yaml.v3",
					Symbol:               &SymbolLookup{Name: "Node"},
					Kind:                 LookupFullPackage,
					ContinueOnSymbolMiss: true,
				})
			}
		})
	}
}

func TestLookupCandidatesTwoArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []LookupCandidate
	}{
		{
			name: "symbol",
			args: []string{"encoding/json", "Decode"},
			want: []LookupCandidate{{Package: "encoding/json", UserPath: "encoding/json", Symbol: &SymbolLookup{Name: "Decode"}, Kind: LookupFullPackage}},
		},
		{
			name: "member",
			args: []string{"encoding/json", "Decoder.Decode"},
			want: []LookupCandidate{{Package: "encoding/json", UserPath: "encoding/json", Symbol: &SymbolLookup{Name: "Decoder", Member: new("Decode")}, Kind: LookupFullPackage}},
		},
		{
			name: "field",
			args: []string{"net/http", "Request.Method"},
			want: []LookupCandidate{{Package: "net/http", UserPath: "net/http", Symbol: &SymbolLookup{Name: "Request", Member: new("Method")}, Kind: LookupFullPackage}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lookupCandidates(tt.args)
			if err != nil {
				t.Fatalf("lookupCandidates(%q) error = %v", tt.args, err)
			}
			assertCandidatePrefix(t, got, tt.want)
		})
	}
}

func TestLookupCandidatesCurrentPackage(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []LookupCandidate
	}{
		{
			name: "no_args",
			want: []LookupCandidate{{Kind: LookupCurrentPackage}},
		},
		{
			name: "symbol",
			args: []string{"Foo"},
			want: []LookupCandidate{{Symbol: &SymbolLookup{Name: "Foo"}, Kind: LookupCurrentPackage}},
		},
		{
			name: "member",
			args: []string{"Foo.Bar"},
			want: []LookupCandidate{{Symbol: &SymbolLookup{Name: "Foo", Member: new("Bar")}, Kind: LookupCurrentPackage}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lookupCandidates(tt.args)
			if err != nil {
				t.Fatalf("lookupCandidates(%q) error = %v", tt.args, err)
			}
			assertCandidatePrefix(t, got, tt.want)
		})
	}
}

func TestLookupCandidatesArity(t *testing.T) {
	for _, args := range [][]string{nil, {"encoding/json"}, {"encoding/json", "Decoder"}} {
		if _, err := lookupCandidates(args); err != nil {
			t.Fatalf("lookupCandidates(%q) error = %v", args, err)
		}
	}

	_, err := lookupCandidates([]string{"a", "b", "c"})
	if err == nil {
		t.Fatal("lookupCandidates(three args) error = nil, want usage error")
	}
}

func TestLookupResolverTriesWholePackageBeforeDottedSplit(t *testing.T) {
	source := PackageSource{
		ImportPath: "example.com/root/pkg.Type",
		Files: []SourceFile{{
			Name: "type.go",
			Data: []byte(`package Type
`),
		}},
	}
	resolver := LookupResolver{
		Resolver: Resolver{Fetchers: []SourceFetcher{&mapSourceFetcher{sources: map[string]PackageSource{
			"example.com/root/pkg.Type": source,
		}}}},
	}

	got, err := resolver.Resolve(context.Background(), Options{Args: []string{"example.com/root/pkg.Type"}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Source.ImportPath != "example.com/root/pkg.Type" {
		t.Fatalf("Resolve() package = %q, want %q", got.Source.ImportPath, "example.com/root/pkg.Type")
	}
	if got.Lookup.Symbol != nil {
		t.Fatalf("Resolve() symbol = %#v, want nil", got.Lookup.Symbol)
	}
}

func TestLookupResolverDefersLiteralResolutionErrorForDottedLookup(t *testing.T) {
	fetcher := &mapSourceFetcher{
		errs: map[string]error{
			"example.com/root/pkg.Type": errors.New("cloning literal path: authentication required"),
		},
		sources: map[string]PackageSource{
			"example.com/root/pkg": {
				ImportPath: "example.com/root/pkg",
				Files: []SourceFile{{
					Name: "pkg.go",
					Data: []byte("package pkg\n\ntype Type struct{}\n"),
				}},
			},
		},
	}
	resolver := LookupResolver{Resolver: Resolver{Fetchers: []SourceFetcher{fetcher}}}

	got, err := resolver.Resolve(context.Background(), Options{Args: []string{"example.com/root/pkg.Type"}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := got.Source.ImportPath, "example.com/root/pkg"; got != want {
		t.Fatalf("Resolve() package = %q, want %q", got, want)
	}
	if got.Lookup.Symbol == nil || got.Lookup.Symbol.Name != "Type" {
		t.Fatalf("Resolve() symbol = %#v, want Type", got.Lookup.Symbol)
	}
}

func TestLookupResolverDoesNotDeferParseErrorForDottedLookup(t *testing.T) {
	resolver := LookupResolver{Resolver: Resolver{Fetchers: []SourceFetcher{&mapSourceFetcher{
		sources: map[string]PackageSource{
			"example.com/root/pkg.Type": {
				ImportPath: "example.com/root/pkg.Type",
				Files: []SourceFile{{
					Name: "broken.go",
					Data: []byte("package\n"),
				}},
			},
			"example.com/root/pkg": {
				ImportPath: "example.com/root/pkg",
				Files: []SourceFile{{
					Name: "pkg.go",
					Data: []byte("package pkg\n\ntype Type struct{}\n"),
				}},
			},
		},
	}}}}

	_, err := resolver.Resolve(context.Background(), Options{Args: []string{"example.com/root/pkg.Type"}})
	if err == nil {
		t.Fatal("Resolve() error = nil, want parse error")
	}
	assertErrorNotContains(t, err, "no symbol Type")
}

func TestLookupResolverPrefersNormalizedResolutionErrorAfterDottedLookupMiss(t *testing.T) {
	resolver := LookupResolver{Resolver: Resolver{Fetchers: []SourceFetcher{&mapSourceFetcher{
		errs: map[string]error{
			"example.com/root/pkg.Type": errors.New("cloning literal path: authentication required"),
			"example.com/root/pkg":      errors.New("cloning normalized package: authentication required"),
		},
	}}}}

	_, err := resolver.Resolve(context.Background(), Options{Args: []string{"example.com/root/pkg.Type"}})
	assertErrorContains(t, err, "cloning normalized package")
	assertErrorNotContains(t, err, "cloning literal path")
}

func TestLookupResolverReturnsCachedPackageAbsenceWithoutRemoteFallback(t *testing.T) {
	modPath := "github.com/private/mod"
	fallback := &mapSourceFetcher{errs: map[string]error{
		modPath: errors.New("remote fallback should not run"),
	}}
	resolver := LookupResolver{Resolver: Resolver{Fetchers: []SourceFetcher{
		&mapSourceFetcher{errs: map[string]error{
			modPath: &cachedPackageAbsentError{
				Module:  module.Version{Path: modPath, Version: "v1.0.0"},
				Package: modPath,
			},
		}},
		fallback,
	}}}

	_, err := resolver.Resolve(context.Background(), Options{
		Args:    []string{modPath},
		Version: "v1.0.0",
	})
	assertErrorContains(t, err, "package "+modPath+" not found")
	assertErrorNotContains(t, err, "remote fallback")
}

func TestLookupResolverUsesCurrentPackageDirectory(t *testing.T) {
	source := PackageSource{
		ImportPath: "example.com/root/internal/godoc",
		Files: []SourceFile{{
			Name: "godoc.go",
			Data: []byte(`package godoc
`),
		}},
	}
	fetcher := &mapSourceFetcher{sources: map[string]PackageSource{
		"./internal/godoc": source,
	}}
	resolver := LookupResolver{
		Resolver: Resolver{Fetchers: []SourceFetcher{fetcher}},
		Current: CurrentPackage{
			WorkDir:    "/repo/internal/godoc",
			ModuleDir:  "/repo",
			ModulePath: "example.com/root",
		},
	}

	got, err := resolver.Resolve(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Source.ImportPath != "example.com/root/internal/godoc" {
		t.Fatalf("Resolve() package = %q, want current package import path", got.Source.ImportPath)
	}
}

func TestLookupResolverContinuesWhenSymbolMissing(t *testing.T) {
	resolver := LookupResolver{
		Resolver: Resolver{Fetchers: []SourceFetcher{&mapSourceFetcher{sources: map[string]PackageSource{
			"crypto/rand": {
				ImportPath: "crypto/rand",
				Files: []SourceFile{{
					Name: "rand.go",
					Data: []byte(`package rand

func Read() {}
`),
				}},
			},
			"math/rand": {
				ImportPath: "math/rand",
				Files: []SourceFile{{
					Name: "rand.go",
					Data: []byte(`package rand

func Float64() float64 { return 0 }
`),
				}},
			},
		}}}},
		PackageIndex: staticPackageIndex{matches: map[string][]IndexedPackage{
			"rand": {
				{ImportPath: "crypto/rand"},
				{ImportPath: "math/rand"},
			},
		}},
	}

	got, err := resolver.Resolve(context.Background(), Options{Args: []string{"rand.Float64"}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Source.ImportPath != "math/rand" {
		t.Fatalf("Resolve() package = %q, want math/rand", got.Source.ImportPath)
	}
	wantAttempts := []string{"rand.Float64", "crypto/rand", "math/rand"}
	if !reflect.DeepEqual(got.Attempts, wantAttempts) {
		t.Fatalf("Resolve() attempts = %#v, want %#v", got.Attempts, wantAttempts)
	}
}

func TestLookupResolverContinuesPastBuildExcludedSuffixMatch(t *testing.T) {
	context := build.Default
	context.GOOS = "linux"
	resolver := LookupResolver{
		Resolver: Resolver{Fetchers: []SourceFetcher{&mapSourceFetcher{sources: map[string]PackageSource{
			"example.com/first/widget": {
				ImportPath: "example.com/first/widget",
				Files: []SourceFile{{
					Name: "widget_windows.go",
					Data: []byte("package widget\n\nfunc Active() {}\n"),
				}},
			},
			"example.com/second/widget": {
				ImportPath: "example.com/second/widget",
				Files: []SourceFile{{
					Name: "widget_linux.go",
					Data: []byte("package widget\n\nfunc Active() {}\n"),
				}},
			},
		}}}},
		PackageIndex: staticPackageIndex{matches: map[string][]IndexedPackage{
			"widget": {
				{ImportPath: "example.com/first/widget"},
				{ImportPath: "example.com/second/widget"},
			},
		}},
		BuildContext: SourceBuildContext{Context: context},
	}

	got, err := resolver.Resolve(t.Context(), Options{Args: []string{"widget", "Active"}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Source.ImportPath != "example.com/second/widget" {
		t.Fatalf("Resolve() package = %q, want active suffix match", got.Source.ImportPath)
	}
}

func TestLookupResolverReportsSymbolMissAcrossSuffixCandidates(t *testing.T) {
	resolver := LookupResolver{
		Resolver: Resolver{Fetchers: []SourceFetcher{&mapSourceFetcher{sources: map[string]PackageSource{
			"crypto/rand": {
				ImportPath: "crypto/rand",
				Files: []SourceFile{{
					Name: "rand.go",
					Data: []byte(`package rand

func Read() {}
`),
				}},
			},
			"math/rand": {
				ImportPath: "math/rand",
				Files: []SourceFile{{
					Name: "rand.go",
					Data: []byte(`package rand

func Float64() float64 { return 0 }
`),
				}},
			},
		}}}},
		PackageIndex: staticPackageIndex{matches: map[string][]IndexedPackage{
			"rand": {
				{ImportPath: "crypto/rand"},
				{ImportPath: "math/rand"},
			},
		}},
	}

	_, err := resolver.Resolve(context.Background(), Options{Args: []string{"rand.Missing"}})
	if err == nil {
		t.Fatal("Resolve() error = nil, want symbol miss")
	}
	assertErrorContains(t, err, "no symbol Missing")
	assertErrorContains(t, err, "crypto/rand")
	assertErrorContains(t, err, "math/rand")
	assertErrorNotContains(t, err, "source fetcher not applicable")
}

func TestLookupResolverReportsPackageMissForPackageOnlyLookup(t *testing.T) {
	resolver := LookupResolver{
		Resolver: Resolver{Fetchers: []SourceFetcher{&mapSourceFetcher{sources: map[string]PackageSource{}}}},
	}

	_, err := resolver.Resolve(context.Background(), Options{Args: []string{"example.com/missing"}})
	if err == nil {
		t.Fatal("Resolve() error = nil, want package miss")
	}
	if !strings.Contains(err.Error(), "package example.com/missing not found") {
		t.Fatalf("Resolve() error = %v, want package not found", err)
	}
	assertErrorNotContains(t, err, "source fetcher not applicable")
}

type mapSourceFetcher struct {
	sources map[string]PackageSource
	errs    map[string]error
}

func (f *mapSourceFetcher) Fetch(_ context.Context, pkg string, _ Options) (PackageSource, error) {
	if err, ok := f.errs[pkg]; ok {
		return PackageSource{}, err
	}
	source, ok := f.sources[pkg]
	if !ok {
		return PackageSource{}, ErrSourceNotApplicable
	}
	return source, nil
}

type staticPackageIndex struct {
	matches map[string][]IndexedPackage
	err     error
}

func (i staticPackageIndex) MatchSuffix(suffix string) ([]IndexedPackage, error) {
	if i.err != nil {
		return nil, i.err
	}
	return i.matches[suffix], nil
}

func assertCandidatePrefix(t *testing.T, got, want []LookupCandidate) {
	t.Helper()
	if len(got) < len(want) {
		t.Fatalf("lookupCandidates() returned %d candidates, want at least %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if !sameCandidate(got[i], want[i]) {
			t.Fatalf("candidate %d = %#v, want %#v; all candidates: %#v", i, got[i], want[i], got)
		}
	}
}

func assertCandidateContains(t *testing.T, got []LookupCandidate, want LookupCandidate) {
	t.Helper()
	for _, candidate := range got {
		if sameCandidate(candidate, want) {
			return
		}
	}
	t.Fatalf("lookupCandidates() missing candidate %#v; all candidates: %#v", want, got)
}

func sameCandidate(a, b LookupCandidate) bool {
	return a.Package == b.Package &&
		a.UserPath == b.UserPath &&
		a.Kind == b.Kind &&
		a.ContinueOnSymbolMiss == b.ContinueOnSymbolMiss &&
		reflect.DeepEqual(a.Symbol, b.Symbol)
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want containing %q", err, want)
	}
}

func assertErrorNotContains(t *testing.T, err error, want string) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want not containing %q", err, want)
	}
}

var _ SourceFetcher = (*mapSourceFetcher)(nil)
var _ PackageIndex = staticPackageIndex{}
