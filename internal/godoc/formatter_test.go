package godoc

import (
	"go/doc"
	"strings"
	"testing"
)

const formatterTestSrc = `// Package example provides utilities for testing.
package example

// Widget represents a test widget.
type Widget struct {
	// Name is the widget name.
	Name string
}

// NewWidget creates a new Widget with the given name.
func NewWidget(name string) *Widget {
	return &Widget{Name: name}
}

// Reset clears the widget name.
func (w *Widget) Reset() {
	w.Name = ""
}

// DefaultName is the default widget name.
const DefaultName string = "default"

// ErrNotFound is returned when a widget is not found.
var ErrNotFound error

// Free releases all widgets.
func Free() {}
`

func TestFormatPackageDefault(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterTestSrc, 0)

	out, err := FormatPackage(parsed.Package, parsed.Fset, parsed.Aliases, Options{})
	if err != nil {
		t.Fatalf("FormatPackage() error = %v", err)
	}

	assertContains(t, out, `package example // import "example.com/test"`)
	assertContains(t, out, "Package example provides utilities")
	assertContains(t, out, "const DefaultName string = \"default\"")
	assertContains(t, out, "var ErrNotFound error")
	assertContains(t, out, "func Free()")
	assertContains(t, out, "type Widget struct")
	assertContains(t, out, "func NewWidget(name string) *Widget")
	assertContains(t, out, "func (w *Widget) Reset()")
}

func TestFormatPackageShortOmitsDocs(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterTestSrc, 0)

	out, err := FormatPackage(parsed.Package, parsed.Fset, parsed.Aliases, Options{Short: true})
	if err != nil {
		t.Fatalf("FormatPackage() error = %v", err)
	}

	assertContains(t, out, "func NewWidget(name string) *Widget")
	assertNotContains(t, out, "creates a new Widget")
}

func TestFormatPackageAllIncludesDocs(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterTestSrc, 0)

	out, err := FormatPackage(parsed.Package, parsed.Fset, parsed.Aliases, Options{All: true})
	if err != nil {
		t.Fatalf("FormatPackage() error = %v", err)
	}

	assertContains(t, out, "Widget represents a test widget")
	assertContains(t, out, "NewWidget creates a new Widget")
	assertContains(t, out, "Reset clears the widget name")
}

func TestFormatPackageSymbolFunc(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterTestSrc, 0)

	out, err := FormatPackage(parsed.Package, parsed.Fset, parsed.Aliases, Options{Symbol: "Free"})
	if err != nil {
		t.Fatalf("FormatPackage() error = %v", err)
	}

	assertContains(t, out, "func Free()")
	assertContains(t, out, "Free releases all widgets")
}

func TestFormatPackageSymbolTypeIncludesConstructorsAndMethods(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterTestSrc, 0)

	out, err := FormatPackage(parsed.Package, parsed.Fset, parsed.Aliases, Options{Symbol: "Widget"})
	if err != nil {
		t.Fatalf("FormatPackage() error = %v", err)
	}

	assertContains(t, out, "type Widget struct")
	assertContains(t, out, "Widget represents a test widget")
	assertContains(t, out, "func NewWidget(name string) *Widget")
	assertContains(t, out, "func (w *Widget) Reset()")
}

func TestFormatPackageSymbolMethod(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterTestSrc, 0)

	out, err := FormatPackage(parsed.Package, parsed.Fset, parsed.Aliases, Options{Symbol: "Reset"})
	if err != nil {
		t.Fatalf("FormatPackage() error = %v", err)
	}

	assertContains(t, out, "func (w *Widget) Reset()")
	assertContains(t, out, "Reset clears the widget name")
}

func TestFormatPackageSymbolConst(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterTestSrc, 0)

	out, err := FormatPackage(parsed.Package, parsed.Fset, parsed.Aliases, Options{Symbol: "DefaultName"})
	if err != nil {
		t.Fatalf("FormatPackage() error = %v", err)
	}

	assertContains(t, out, "const DefaultName string = \"default\"")
	assertContains(t, out, "DefaultName is the default widget name")
}

func TestFormatPackageSymbolVar(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterTestSrc, 0)

	out, err := FormatPackage(parsed.Package, parsed.Fset, parsed.Aliases, Options{Symbol: "ErrNotFound"})
	if err != nil {
		t.Fatalf("FormatPackage() error = %v", err)
	}

	assertContains(t, out, "var ErrNotFound error")
	assertContains(t, out, "ErrNotFound is returned")
}

func TestFormatPackageCaseSensitiveMiss(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterTestSrc, 0)

	_, err := FormatPackage(parsed.Package, parsed.Fset, parsed.Aliases, Options{
		Symbol:        "widget",
		CaseSensitive: true,
	})
	if err == nil {
		t.Fatal("FormatPackage() error = nil, want missing symbol error")
	}
}

func TestFormatPackageSourceOutput(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterTestSrc, doc.PreserveAST)

	out, err := FormatPackage(parsed.Package, parsed.Fset, parsed.Aliases, Options{
		Symbol: "NewWidget",
		Src:    true,
	})
	if err != nil {
		t.Fatalf("FormatPackage() error = %v", err)
	}

	assertContains(t, out, "return &Widget{Name: name}")
}

func TestFormatPackageAliasHint(t *testing.T) {
	parsed := parseFormatterTestPackage(t, `package example

import (
	jsonv2 "encoding/json/v2"
	"encoding/json"
)

func Encode() {
	_ = jsonv2.Marshal
	_ = json.Decoder{}
}
`, 0)

	_, err := FormatPackage(parsed.Package, parsed.Fset, parsed.Aliases, Options{Symbol: "jsonv2.Marshal"})
	if err == nil {
		t.Fatal("FormatPackage() error = nil, want alias hint")
	}
	assertContains(t, err.Error(), "hint:")
	assertContains(t, err.Error(), "jsonv2 is an import alias for encoding/json/v2")
	assertContains(t, err.Error(), "scut gotools doc encoding/json/v2 Marshal")

	_, err = FormatPackage(parsed.Package, parsed.Fset, parsed.Aliases, Options{Symbol: "json.Decoder"})
	if err == nil {
		t.Fatal("FormatPackage() error = nil, want default import alias hint")
	}
	assertContains(t, err.Error(), "json is an import alias for encoding/json")
	assertContains(t, err.Error(), "scut gotools doc encoding/json Decoder")
}

func parseFormatterTestPackage(t *testing.T, src string, mode doc.Mode) *ParsedPackage {
	t.Helper()
	parsed, err := ParsePackage("example.com/test", []SourceFile{{Name: "test.go", Data: []byte(src)}}, mode)
	if err != nil {
		t.Fatalf("ParsePackage() error = %v", err)
	}
	return parsed
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected output to contain %q:\n%s", want, got)
	}
}

func assertNotContains(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Fatalf("expected output not to contain %q:\n%s", want, got)
	}
}
