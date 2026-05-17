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

// DefaultWidget is the default widget.
var DefaultWidget Widget

// Free releases all widgets.
func Free() {}
`

const formatterMemberTestSrc = `// Package example provides utilities for testing.
package example

type ResponseWriter interface{}

// Request is an HTTP request.
type Request struct {
	// Method specifies the HTTP method.
	Method string
	URL string
}

// Handler responds to a request.
type Handler interface {
	// ServeHTTP handles a request.
	ServeHTTP(ResponseWriter, *Request)
	Hidden()
}

// Widget represents a test widget.
type Widget struct{}

// Reset clears the widget.
func (w *Widget) Reset() {}

// CaseMatch is one spelling.
func CaseMatch() {}

// Casematch is another spelling.
func Casematch() {}
`

const formatterUnexportedTestSrc = `// Package example provides utilities for testing.
package example

// Widget represents a test widget.
type Widget struct {
	// Name is visible.
	Name string
	// hidden is private state.
	hidden string
}

// Exported is visible.
func Exported() {}

// hiddenFunc is hidden.
func hiddenFunc() {}
`

const formatterMainTestSrc = `// Command example runs the example tool.
package main

// Run executes the command.
func Run() {}
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

	assertNotContains(t, out, `package example // import "example.com/test"`)
	assertContains(t, out, "func NewWidget(name string) *Widget")
	assertNotContains(t, out, "creates a new Widget")
}

func TestFormatPackageShortSymbolOmitsHeader(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterTestSrc, 0)

	out, err := FormatLookup(parsed, Lookup{Package: "example.com/test", Symbol: &SymbolLookup{Name: "Widget"}}, Options{Short: true})
	if err != nil {
		t.Fatalf("FormatLookup() error = %v", err)
	}

	assertNotContains(t, out, `package example // import "example.com/test"`)
	assertContains(t, out, "type Widget struct")
	assertContains(t, out, "func NewWidget(name string) *Widget")
	assertContains(t, out, "func (w *Widget) Reset()")
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

func TestFormatPackageAllSymbolIncludesAssociatedDocs(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterTestSrc, 0)

	out, err := FormatLookup(parsed, Lookup{Package: "example.com/test", Symbol: &SymbolLookup{Name: "Widget"}}, Options{All: true})
	if err != nil {
		t.Fatalf("FormatLookup() error = %v", err)
	}

	assertContains(t, out, "DefaultWidget is the default widget")
	assertContains(t, out, "NewWidget creates a new Widget")
	assertContains(t, out, "Reset clears the widget name")
}

func TestFormatPackageSymbolFunc(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterTestSrc, 0)

	out, err := FormatLookup(parsed, Lookup{Package: "example.com/test", Symbol: &SymbolLookup{Name: "Free"}}, Options{})
	if err != nil {
		t.Fatalf("FormatLookup() error = %v", err)
	}

	assertHasPrefix(t, out, "package example // import \"example.com/test\"\n\nfunc Free()")
	assertContains(t, out, "func Free()")
	assertContains(t, out, "Free releases all widgets")
}

func TestFormatPackageSymbolIncludesHeader(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterTestSrc, 0)

	out, err := FormatLookup(parsed, Lookup{Package: "example.com/test", Symbol: &SymbolLookup{Name: "Free"}}, Options{})
	if err != nil {
		t.Fatalf("FormatLookup() error = %v", err)
	}

	assertHasPrefix(t, out, "package example // import \"example.com/test\"\n\nfunc Free()")
}

func TestFormatPackageTypeDocSeparatesDeclarationAndDoc(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterTestSrc, 0)

	out, err := FormatLookup(parsed, Lookup{Package: "example.com/test", Symbol: &SymbolLookup{Name: "Widget"}}, Options{})
	if err != nil {
		t.Fatalf("FormatLookup() error = %v", err)
	}

	assertContains(t, out, "}\n    Widget represents a test widget")
}

func TestFormatPackageSymbolTypeIncludesConstructorsAndMethods(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterTestSrc, 0)

	out, err := FormatLookup(parsed, Lookup{Package: "example.com/test", Symbol: &SymbolLookup{Name: "Widget"}}, Options{})
	if err != nil {
		t.Fatalf("FormatLookup() error = %v", err)
	}

	assertContains(t, out, "type Widget struct")
	assertContains(t, out, "Widget represents a test widget")
	assertContains(t, out, "var DefaultWidget Widget")
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

func TestFormatPackageMethodByTypeAndMember(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterMemberTestSrc, 0)

	out, err := FormatLookup(parsed, Lookup{Package: "example.com/test", Symbol: &SymbolLookup{Name: "Widget", Member: new("Reset")}}, Options{})
	if err != nil {
		t.Fatalf("FormatLookup() error = %v", err)
	}

	assertContains(t, out, "func (w *Widget) Reset()")
	assertContains(t, out, "Reset clears the widget")
	assertNotContains(t, out, "type Widget struct")
}

func TestFormatPackageMethodByPackageLevelShorthand(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterMemberTestSrc, 0)

	out, err := FormatLookup(parsed, Lookup{Package: "example.com/test", Symbol: &SymbolLookup{Name: "reset"}}, Options{})
	if err != nil {
		t.Fatalf("FormatLookup() error = %v", err)
	}

	assertContains(t, out, "func (w *Widget) Reset()")
	assertContains(t, out, "Reset clears the widget")
}

func TestFormatPackageInterfaceMethod(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterMemberTestSrc, 0)

	out, err := FormatLookup(parsed, Lookup{Package: "example.com/test", Symbol: &SymbolLookup{Name: "Handler", Member: new("ServeHTTP")}}, Options{})
	if err != nil {
		t.Fatalf("FormatLookup() error = %v", err)
	}

	assertContains(t, out, "type Handler interface {")
	assertContains(t, out, "ServeHTTP(ResponseWriter, *Request)")
	assertContains(t, out, "}")
	assertNotContains(t, out, "Hidden()")
}

func TestFormatPackageStructField(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterMemberTestSrc, 0)

	out, err := FormatLookup(parsed, Lookup{Package: "example.com/test", Symbol: &SymbolLookup{Name: "Request", Member: new("Method")}}, Options{})
	if err != nil {
		t.Fatalf("FormatLookup() error = %v", err)
	}

	assertContains(t, out, "type Request struct {")
	assertContains(t, out, "// Method specifies the HTTP method.")
	assertContains(t, out, "Method string")
	assertContains(t, out, "// ... other fields elided ...")
	assertContains(t, out, "}")
	assertNotContains(t, out, "URL string")
}

func TestMatchDocName(t *testing.T) {
	tests := []struct {
		query string
		name  string
		opts  Options
		want  bool
	}{
		{query: "decode", name: "Decode", want: true},
		{query: "Decode", name: "decode", want: false},
		{query: "Decode", name: "Decode", want: true},
		{query: "decode", name: "Decode", opts: Options{CaseSensitive: true}, want: false},
	}

	for _, tt := range tests {
		got := matchDocName(tt.query, tt.name, tt.opts)
		if got != tt.want {
			t.Fatalf("matchDocName(%q, %q, %#v) = %v, want %v", tt.query, tt.name, tt.opts, got, tt.want)
		}
	}
}

func TestFormatPackageSymbolPrintsMultipleLooseMatches(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterMemberTestSrc, 0)

	out, err := FormatLookup(parsed, Lookup{Package: "example.com/test", Symbol: &SymbolLookup{Name: "casematch"}}, Options{})
	if err != nil {
		t.Fatalf("FormatLookup() error = %v", err)
	}

	assertContains(t, out, "func CaseMatch()")
	assertContains(t, out, "func Casematch()")
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

	assertContains(t, out, `package example // import "example.com/test"`)
	assertContains(t, out, "// NewWidget creates a new Widget with the given name.")
	assertContains(t, out, "return &Widget{Name: name}")
	assertNotContains(t, out, "    NewWidget creates a new Widget")
}

func TestFormatPackageMainHidesSymbolsUnlessCmd(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterMainTestSrc, 0)

	out, err := FormatLookup(parsed, Lookup{Package: "example.com/test"}, Options{})
	if err != nil {
		t.Fatalf("FormatLookup() error = %v", err)
	}
	assertContains(t, out, "Command example runs the example tool")
	assertNotContains(t, out, "func Run()")

	out, err = FormatLookup(parsed, Lookup{Package: "example.com/test"}, Options{Cmd: true})
	if err != nil {
		t.Fatalf("FormatLookup() with Cmd error = %v", err)
	}
	assertContains(t, out, "func Run()")
}

func TestFormatPackageFiltersUnexportedByDefault(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterUnexportedTestSrc, 0)

	out, err := FormatLookup(parsed, Lookup{Package: "example.com/test"}, Options{})
	if err != nil {
		t.Fatalf("FormatLookup() error = %v", err)
	}
	assertContains(t, out, "func Exported()")
	assertNotContains(t, out, "func hiddenFunc()")

	out, err = FormatLookup(parsed, Lookup{Package: "example.com/test", Symbol: &SymbolLookup{Name: "Widget"}}, Options{})
	if err != nil {
		t.Fatalf("FormatLookup() symbol error = %v", err)
	}
	assertContains(t, out, "Name string")
	assertContains(t, out, "\t// Has unexported fields.\n")
	assertNotContains(t, out, "\t\t// Has unexported fields.")
	assertNotContains(t, out, "hidden string")
}

func TestFormatPackageUnexportedFlagShowsUnexported(t *testing.T) {
	parsed := parseFormatterTestPackage(t, formatterUnexportedTestSrc, 0)

	out, err := FormatLookup(parsed, Lookup{Package: "example.com/test"}, Options{Unexported: true})
	if err != nil {
		t.Fatalf("FormatLookup() error = %v", err)
	}
	assertContains(t, out, "func hiddenFunc()")

	out, err = FormatLookup(parsed, Lookup{Package: "example.com/test", Symbol: &SymbolLookup{Name: "Widget"}}, Options{Unexported: true})
	if err != nil {
		t.Fatalf("FormatLookup() symbol error = %v", err)
	}
	assertContains(t, out, "hidden string")
	assertNotContains(t, out, "// Has unexported fields.")
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
	mode |= doc.AllDecls
	parsed, err := ParsePackage("example.com/test", []SourceFile{{Name: "test.go", Data: []byte(src)}}, mode)
	if err != nil {
		t.Fatalf("ParsePackage() error = %v", err)
	}
	return parsed
}

func assertHasPrefix(t *testing.T, got, want string) {
	t.Helper()
	if !strings.HasPrefix(got, want) {
		t.Fatalf("expected output to start with %q:\n%s", want, got)
	}
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
