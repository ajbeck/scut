package godoc

import (
	"errors"
	"go/doc"
	"testing"
)

func TestParsePackageBuildsExportedDocs(t *testing.T) {
	parsed, err := ParsePackage("example.com/widgets", []SourceFile{
		{
			Name: "widgets.go",
			Data: []byte(`// Package widgets manages widgets.
package widgets

// New returns a widget.
func New() {}

// hidden is internal.
func hidden() {}
`),
		},
	}, 0)
	if err != nil {
		t.Fatalf("ParsePackage() error = %v", err)
	}

	if got, want := parsed.Package.Name, "widgets"; got != want {
		t.Fatalf("Package.Name = %q, want %q", got, want)
	}
	if got, want := parsed.Package.Doc, "Package widgets manages widgets.\n"; got != want {
		t.Fatalf("Package.Doc = %q, want %q", got, want)
	}
	if got, want := len(parsed.Package.Funcs), 1; got != want {
		t.Fatalf("len(Funcs) = %d, want %d", got, want)
	}
	if got, want := parsed.Package.Funcs[0].Name, "New"; got != want {
		t.Fatalf("Funcs[0].Name = %q, want %q", got, want)
	}
}

func TestParsePackageIncludesUnexportedWithAllDecls(t *testing.T) {
	parsed, err := ParsePackage("example.com/widgets", []SourceFile{
		{
			Name: "widgets.go",
			Data: []byte(`package widgets

// hidden is internal.
func hidden() {}
`),
		},
	}, doc.AllDecls)
	if err != nil {
		t.Fatalf("ParsePackage() error = %v", err)
	}

	if got, want := len(parsed.Package.Funcs), 1; got != want {
		t.Fatalf("len(Funcs) = %d, want %d", got, want)
	}
	if got, want := parsed.Package.Funcs[0].Name, "hidden"; got != want {
		t.Fatalf("Funcs[0].Name = %q, want %q", got, want)
	}
}

func TestParsePackageCollectsImportAliases(t *testing.T) {
	parsed, err := ParsePackage("example.com/widgets", []SourceFile{
		{
			Name: "widgets.go",
			Data: []byte(`package widgets

import (
	jsonv2 "encoding/json/v2"
	plain "example.com/plain"
)

func Encode() {
	_ = jsonv2.Marshal
	_ = plain.Name
}
`),
		},
	}, 0)
	if err != nil {
		t.Fatalf("ParsePackage() error = %v", err)
	}

	if got, want := parsed.Aliases["jsonv2"], "encoding/json/v2"; got != want {
		t.Fatalf("Aliases[jsonv2] = %q, want %q", got, want)
	}
	if got, want := parsed.Aliases["plain"], "example.com/plain"; got != want {
		t.Fatalf("Aliases[plain] = %q, want %q", got, want)
	}
}

func TestParsePackageSkipsBadFiles(t *testing.T) {
	parsed, err := ParsePackage("example.com/widgets", []SourceFile{
		{
			Name: "bad.go",
			Data: []byte(`package widgets

func broken(
`),
		},
		{
			Name: "good.go",
			Data: []byte(`package widgets

// Good works.
func Good() {}
`),
		},
	}, 0)
	if err != nil {
		t.Fatalf("ParsePackage() error = %v", err)
	}

	if got, want := len(parsed.Package.Funcs), 1; got != want {
		t.Fatalf("len(Funcs) = %d, want %d", got, want)
	}
	if got, want := parsed.Package.Funcs[0].Name, "Good"; got != want {
		t.Fatalf("Funcs[0].Name = %q, want %q", got, want)
	}
}

func TestParsePackageReturnsNoParseableFiles(t *testing.T) {
	_, err := ParsePackage("example.com/widgets", []SourceFile{
		{
			Name: "bad.go",
			Data: []byte(`package widgets

func broken(
`),
		},
	}, 0)
	if !errors.Is(err, ErrNoParseableFiles) {
		t.Fatalf("ParsePackage() error = %v, want ErrNoParseableFiles", err)
	}
}
