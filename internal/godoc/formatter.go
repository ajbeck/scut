package godoc

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/format"
	"go/token"
	"path"
	"strings"
)

// FormatPackage renders package documentation as terminal text.
func FormatPackage(pkg *doc.Package, fset *token.FileSet, aliases ImportAliases, opts Options) (string, error) {
	if opts.Symbol != "" {
		return formatSymbol(pkg, fset, aliases, opts)
	}
	if opts.Short {
		return formatShort(pkg, fset), nil
	}
	if opts.All {
		return formatAll(pkg, fset, opts), nil
	}
	return formatDefault(pkg, fset), nil
}

func formatDefault(pkg *doc.Package, fset *token.FileSet) string {
	var b strings.Builder
	writePackageHeader(&b, pkg)
	writePackageDoc(&b, pkg)
	writeValueSummaries(&b, fset, pkg.Consts)
	writeValueSummaries(&b, fset, pkg.Vars)
	writeFuncSummaries(&b, fset, pkg.Funcs)
	writeTypeSummaries(&b, fset, pkg.Types)
	return strings.TrimRight(b.String(), "\n")
}

func formatShort(pkg *doc.Package, fset *token.FileSet) string {
	var b strings.Builder
	writePackageHeader(&b, pkg)
	writeValueSummaries(&b, fset, pkg.Consts)
	writeValueSummaries(&b, fset, pkg.Vars)
	writeFuncSummaries(&b, fset, pkg.Funcs)
	writeTypeSummaries(&b, fset, pkg.Types)
	return strings.TrimRight(b.String(), "\n")
}

func formatAll(pkg *doc.Package, fset *token.FileSet, opts Options) string {
	var b strings.Builder
	writePackageHeader(&b, pkg)
	writePackageDoc(&b, pkg)
	writeValues(&b, pkg, fset, pkg.Consts)
	writeValues(&b, pkg, fset, pkg.Vars)
	for _, fn := range pkg.Funcs {
		b.WriteString("\n")
		writeFunc(&b, pkg, fset, fn, opts)
	}
	for _, typ := range pkg.Types {
		b.WriteString("\n")
		writeType(&b, pkg, fset, typ)
		for _, fn := range typ.Funcs {
			b.WriteString("\n")
			writeFunc(&b, pkg, fset, fn, opts)
		}
		for _, method := range typ.Methods {
			b.WriteString("\n")
			writeFunc(&b, pkg, fset, method, opts)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatSymbol(pkg *doc.Package, fset *token.FileSet, aliases ImportAliases, opts Options) (string, error) {
	matches := strings.EqualFold
	if opts.CaseSensitive {
		matches = func(a, b string) bool { return a == b }
	}

	var b strings.Builder
	for _, fn := range pkg.Funcs {
		if matches(fn.Name, opts.Symbol) {
			writeFunc(&b, pkg, fset, fn, opts)
			return strings.TrimRight(b.String(), "\n"), nil
		}
	}
	for _, typ := range pkg.Types {
		if matches(typ.Name, opts.Symbol) {
			writeType(&b, pkg, fset, typ)
			for _, fn := range typ.Funcs {
				b.WriteString("\n")
				b.WriteString(formatFuncSig(fset, fn))
			}
			for _, method := range typ.Methods {
				b.WriteString("\n")
				b.WriteString(formatFuncSig(fset, method))
			}
			return strings.TrimRight(b.String(), "\n"), nil
		}
		for _, fn := range typ.Funcs {
			if matches(fn.Name, opts.Symbol) {
				writeFunc(&b, pkg, fset, fn, opts)
				return strings.TrimRight(b.String(), "\n"), nil
			}
		}
		for _, method := range typ.Methods {
			if matches(method.Name, opts.Symbol) {
				writeFunc(&b, pkg, fset, method, opts)
				return strings.TrimRight(b.String(), "\n"), nil
			}
		}
	}
	for _, value := range pkg.Consts {
		if valueMatches(value, opts.Symbol, matches) {
			writeValue(&b, pkg, fset, value)
			return strings.TrimRight(b.String(), "\n"), nil
		}
	}
	for _, value := range pkg.Vars {
		if valueMatches(value, opts.Symbol, matches) {
			writeValue(&b, pkg, fset, value)
			return strings.TrimRight(b.String(), "\n"), nil
		}
	}

	if prefix, suffix, ok := strings.Cut(opts.Symbol, "."); ok && aliases != nil {
		if importPath, found := aliases[prefix]; found {
			return "", fmt.Errorf("symbol %q not found in package %s\n\nhint: %s is an import alias for %s\ntry: scut gotools doc %s %s", opts.Symbol, pkg.Name, prefix, importPath, importPath, suffix)
		}
	}

	return "", fmt.Errorf("symbol %q not found in package %s", opts.Symbol, pkg.Name)
}

func writePackageHeader(b *strings.Builder, pkg *doc.Package) {
	fmt.Fprintf(b, "package %s // import %q\n", pkg.Name, pkg.ImportPath)
}

func writePackageDoc(b *strings.Builder, pkg *doc.Package) {
	if pkg.Doc == "" {
		return
	}
	b.WriteString("\n")
	b.WriteString(wrapDoc(pkg, pkg.Doc))
}

func writeValueSummaries(b *strings.Builder, fset *token.FileSet, values []*doc.Value) {
	for _, value := range values {
		b.WriteString("\n")
		b.WriteString(formatDeclOneLine(fset, value.Decl))
	}
}

func writeFuncSummaries(b *strings.Builder, fset *token.FileSet, funcs []*doc.Func) {
	for _, fn := range funcs {
		b.WriteString("\n")
		b.WriteString(formatFuncSig(fset, fn))
	}
}

func writeTypeSummaries(b *strings.Builder, fset *token.FileSet, types []*doc.Type) {
	for _, typ := range types {
		b.WriteString("\n")
		b.WriteString(formatDeclOneLine(fset, typ.Decl))
		writeFuncSummaries(b, fset, typ.Funcs)
		writeFuncSummaries(b, fset, typ.Methods)
	}
}

func writeValues(b *strings.Builder, pkg *doc.Package, fset *token.FileSet, values []*doc.Value) {
	for _, value := range values {
		b.WriteString("\n")
		writeValue(b, pkg, fset, value)
	}
}

func writeValue(b *strings.Builder, pkg *doc.Package, fset *token.FileSet, value *doc.Value) {
	b.WriteString(formatDecl(fset, value.Decl))
	if value.Doc != "" {
		b.WriteString(indentDoc(wrapDoc(pkg, value.Doc)))
	}
}

func writeType(b *strings.Builder, pkg *doc.Package, fset *token.FileSet, typ *doc.Type) {
	b.WriteString(formatDecl(fset, typ.Decl))
	if typ.Doc != "" {
		b.WriteString(indentDoc(wrapDoc(pkg, typ.Doc)))
	}
}

func writeFunc(b *strings.Builder, pkg *doc.Package, fset *token.FileSet, fn *doc.Func, opts Options) {
	if opts.Src {
		b.WriteString(formatDecl(fset, fn.Decl))
	} else {
		b.WriteString(formatFuncSig(fset, fn))
	}
	if fn.Doc != "" {
		b.WriteString("\n")
		b.WriteString(indentDoc(wrapDoc(pkg, fn.Doc)))
	}
}

func valueMatches(value *doc.Value, symbol string, matches func(string, string) bool) bool {
	for _, name := range value.Names {
		if matches(name, symbol) {
			return true
		}
	}
	return false
}

func formatDecl(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return fmt.Sprintf("// format error: %v", err)
	}
	return buf.String()
}

func formatDeclOneLine(fset *token.FileSet, node ast.Node) string {
	full := formatDecl(fset, node)
	first, _, multiLine := strings.Cut(full, "\n")
	if multiLine {
		return first + " ..."
	}
	return first
}

func formatFuncSig(fset *token.FileSet, fn *doc.Func) string {
	body := fn.Decl.Body
	fn.Decl.Body = nil
	sig := formatDecl(fset, fn.Decl)
	fn.Decl.Body = body
	return sig
}

func wrapDoc(pkg *doc.Package, text string) string {
	parsed := pkg.Parser().Parse(text)
	return string(pkg.Printer().Text(parsed))
}

func indentDoc(text string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(text, "\n") {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString("    ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func defaultImportAlias(importPath string) string {
	return path.Base(importPath)
}
