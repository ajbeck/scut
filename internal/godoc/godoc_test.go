package godoc

import (
	"context"
	"errors"
	"strings"
	"testing"
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

func TestClientDocRequiresPackage(t *testing.T) {
	client := Client{}

	_, err := client.Doc(context.Background(), Options{})
	if !errors.Is(err, ErrPackageRequired) {
		t.Fatalf("Doc() error = %v, want ErrPackageRequired", err)
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
