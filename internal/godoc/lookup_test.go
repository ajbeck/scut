package godoc

import (
	"context"
	"reflect"
	"strings"
	"testing"
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
				{Package: "encoding/json", UserPath: "encoding/json", Symbol: &SymbolLookup{Name: "Marshal"}, Kind: LookupFullPackage},
			},
		},
		{
			name: "member",
			args: []string{"encoding/json.Decoder.Decode"},
			want: []LookupCandidate{
				{Package: "encoding/json.Decoder.Decode", UserPath: "encoding/json.Decoder.Decode", Kind: LookupFullPackage},
				{Package: "encoding/json", UserPath: "encoding/json", Symbol: &SymbolLookup{Name: "Decoder", Member: new("Decode")}, Kind: LookupFullPackage},
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
					Package:  "gopkg.in/yaml.v3",
					UserPath: "gopkg.in/yaml.v3",
					Symbol:   &SymbolLookup{Name: "Node"},
					Kind:     LookupFullPackage,
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
}

func (f *mapSourceFetcher) Fetch(_ context.Context, pkg string, _ Options) (PackageSource, error) {
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
