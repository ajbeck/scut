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
	"unicode"
	"unicode/utf8"
)

// FormatPackage renders package documentation as terminal text.
func FormatPackage(pkg *doc.Package, fset *token.FileSet, aliases ImportAliases, opts Options) (string, error) {
	parsed := &ParsedPackage{Package: pkg, Fset: fset, Aliases: aliases}
	var lookup Lookup
	if opts.Package != "" {
		lookup.Package = opts.Package
	}
	if opts.Symbol != "" {
		symbol, err := parseSymbolSpec(opts.Symbol)
		if err != nil {
			return "", err
		}
		lookup.Symbol = symbol
	}
	return FormatLookup(parsed, lookup, opts)
}

// FormatLookup renders package documentation for a resolved lookup.
func FormatLookup(parsed *ParsedPackage, lookup Lookup, opts Options) (string, error) {
	pkg := parsed.Package
	fset := parsed.Fset
	if lookup.Symbol != nil {
		return formatLookupSymbol(parsed, lookup, opts)
	}
	if opts.Short {
		return formatShort(pkg, fset, opts), nil
	}
	if opts.All {
		return formatAll(pkg, fset, opts), nil
	}
	return formatDefault(pkg, fset, opts), nil
}

// LookupExists reports whether parsed contains the requested lookup.
func LookupExists(parsed *ParsedPackage, lookup Lookup, opts Options) bool {
	if lookup.Symbol == nil {
		return true
	}
	symbol := lookup.Symbol.Name
	if lookup.Symbol.Member != nil {
		return memberExists(parsed, symbol, *lookup.Symbol.Member, opts)
	}
	for _, fn := range parsed.Package.Funcs {
		if matchDocName(symbol, fn.Name, opts) {
			return true
		}
	}
	for _, typ := range parsed.Package.Types {
		if matchDocName(symbol, typ.Name, opts) {
			return true
		}
		for _, fn := range typ.Funcs {
			if matchDocName(symbol, fn.Name, opts) {
				return true
			}
		}
		for _, method := range typ.Methods {
			if matchDocName(symbol, method.Name, opts) {
				return true
			}
		}
	}
	for _, value := range parsed.Package.Consts {
		if valueMatches(value, symbol, func(name, symbol string) bool { return matchDocName(symbol, name, opts) }) {
			return true
		}
	}
	for _, value := range parsed.Package.Vars {
		if valueMatches(value, symbol, func(name, symbol string) bool { return matchDocName(symbol, name, opts) }) {
			return true
		}
	}
	return false
}

func memberExists(parsed *ParsedPackage, symbol, member string, opts Options) bool {
	for _, typ := range parsed.Package.Types {
		if !matchDocName(symbol, typ.Name, opts) {
			continue
		}
		for _, method := range typ.Methods {
			if matchDocName(member, method.Name, opts) {
				return true
			}
		}
	}
	return interfaceMethodExists(parsed, symbol, member, opts) || structFieldExists(parsed, symbol, member, opts)
}

func formatDefault(pkg *doc.Package, fset *token.FileSet, opts Options) string {
	var b strings.Builder
	writePackageHeader(&b, pkg)
	writePackageDoc(&b, pkg)
	if pkg.Name == "main" && !opts.Cmd {
		return strings.TrimRight(b.String(), "\n")
	}
	writeValueSummaries(&b, fset, pkg.Consts, opts)
	writeValueSummaries(&b, fset, pkg.Vars, opts)
	writeFuncSummaries(&b, fset, pkg.Funcs, opts)
	writeTypeSummaries(&b, fset, pkg.Types, opts)
	return strings.TrimRight(b.String(), "\n")
}

func formatShort(pkg *doc.Package, fset *token.FileSet, opts Options) string {
	var b strings.Builder
	if pkg.Name == "main" && !opts.Cmd {
		return ""
	}
	writeValueSummaries(&b, fset, pkg.Consts, opts)
	writeValueSummaries(&b, fset, pkg.Vars, opts)
	writeFuncSummaries(&b, fset, pkg.Funcs, opts)
	writeTypeSummaries(&b, fset, pkg.Types, opts)
	return strings.TrimRight(b.String(), "\n")
}

func formatAll(pkg *doc.Package, fset *token.FileSet, opts Options) string {
	var b strings.Builder
	if !opts.Short {
		writePackageHeader(&b, pkg)
	}
	writePackageDoc(&b, pkg)
	writeValues(&b, pkg, fset, pkg.Consts, opts)
	writeValues(&b, pkg, fset, pkg.Vars, opts)
	for _, fn := range pkg.Funcs {
		if !isVisibleName(fn.Name, opts) {
			continue
		}
		b.WriteString("\n")
		writeFunc(&b, pkg, fset, fn, opts)
	}
	for _, typ := range pkg.Types {
		if !isVisibleName(typ.Name, opts) {
			continue
		}
		b.WriteString("\n")
		writeType(&b, pkg, fset, typ, opts)
		for _, fn := range typ.Funcs {
			if !isVisibleName(fn.Name, opts) {
				continue
			}
			b.WriteString("\n")
			writeFunc(&b, pkg, fset, fn, opts)
		}
		for _, method := range typ.Methods {
			if !isVisibleName(method.Name, opts) {
				continue
			}
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
			writeType(&b, pkg, fset, typ, opts)
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
			writeValue(&b, pkg, fset, value, opts)
			return strings.TrimRight(b.String(), "\n"), nil
		}
	}
	for _, value := range pkg.Vars {
		if valueMatches(value, opts.Symbol, matches) {
			writeValue(&b, pkg, fset, value, opts)
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

func formatLookupSymbol(parsed *ParsedPackage, lookup Lookup, opts Options) (string, error) {
	var b strings.Builder
	if !opts.Short {
		writePackageHeader(&b, parsed.Package)
		b.WriteString("\n")
	}
	if lookup.Symbol.Member != nil {
		if writeMethodDoc(&b, parsed, lookup, opts) || writeFieldDoc(&b, parsed, lookup, opts) {
			return strings.TrimRight(b.String(), "\n"), nil
		}
		if parsed.Aliases != nil {
			if importPath, found := parsed.Aliases[lookup.Symbol.Name]; found {
				fullSymbol := lookup.Symbol.Name + "." + *lookup.Symbol.Member
				return "", fmt.Errorf("symbol %q not found in package %s\n\nhint: %s is an import alias for %s\ntry: scut gotools doc %s %s", fullSymbol, parsed.Package.Name, lookup.Symbol.Name, importPath, importPath, *lookup.Symbol.Member)
			}
		}
		return "", fmt.Errorf("no method or field %s.%s in package %s", lookup.Symbol.Name, *lookup.Symbol.Member, parsed.Package.Name)
	}
	if writeSymbolDocs(&b, parsed, lookup.Symbol.Name, opts) {
		return strings.TrimRight(b.String(), "\n"), nil
	}

	if prefix, suffix, ok := strings.Cut(lookup.Symbol.Name, "."); ok && parsed.Aliases != nil {
		if importPath, found := parsed.Aliases[prefix]; found {
			return "", fmt.Errorf("symbol %q not found in package %s\n\nhint: %s is an import alias for %s\ntry: scut gotools doc %s %s", lookup.Symbol.Name, parsed.Package.Name, prefix, importPath, importPath, suffix)
		}
	}
	return "", fmt.Errorf("symbol %q not found in package %s", lookup.Symbol.Name, parsed.Package.Name)
}

func writeSymbolDocs(b *strings.Builder, parsed *ParsedPackage, symbol string, opts Options) bool {
	found := false
	for _, fn := range parsed.Package.Funcs {
		if matchDocName(symbol, fn.Name, opts) {
			writeDocSeparator(b, found)
			writeFunc(b, parsed.Package, parsed.Fset, fn, opts)
			found = true
		}
	}
	printedValues := map[*ast.GenDecl]bool{}
	for _, value := range parsed.Package.Consts {
		if valueMatches(value, symbol, func(name, symbol string) bool { return matchDocName(symbol, name, opts) }) {
			writeDocSeparator(b, found)
			writeValueOnce(b, parsed.Package, parsed.Fset, value, opts, printedValues)
			found = true
		}
	}
	for _, value := range parsed.Package.Vars {
		if valueMatches(value, symbol, func(name, symbol string) bool { return matchDocName(symbol, name, opts) }) {
			writeDocSeparator(b, found)
			writeValueOnce(b, parsed.Package, parsed.Fset, value, opts, printedValues)
			found = true
		}
	}
	for _, typ := range parsed.Package.Types {
		if matchDocName(symbol, typ.Name, opts) {
			writeDocSeparator(b, found)
			writeTypeDoc(b, parsed, typ, opts)
			found = true
		}
		for _, fn := range typ.Funcs {
			if matchDocName(symbol, fn.Name, opts) {
				writeDocSeparator(b, found)
				writeFunc(b, parsed.Package, parsed.Fset, fn, opts)
				found = true
			}
		}
	}
	if found {
		return true
	}
	for _, typ := range parsed.Package.Types {
		for _, method := range typ.Methods {
			if matchDocName(symbol, method.Name, opts) {
				writeDocSeparator(b, found)
				writeFunc(b, parsed.Package, parsed.Fset, method, opts)
				found = true
			}
		}
	}
	return found
}

func writeTypeDoc(b *strings.Builder, parsed *ParsedPackage, typ *doc.Type, opts Options) {
	writeType(b, parsed.Package, parsed.Fset, typ, opts)
	for _, value := range typ.Consts {
		if !valueVisible(value, opts) {
			continue
		}
		b.WriteString("\n")
		if opts.All {
			writeValue(b, parsed.Package, parsed.Fset, value, opts)
			continue
		}
		b.WriteString(formatDeclOneLine(parsed.Fset, value.Decl))
	}
	for _, value := range typ.Vars {
		if !valueVisible(value, opts) {
			continue
		}
		b.WriteString("\n")
		if opts.All {
			writeValue(b, parsed.Package, parsed.Fset, value, opts)
			continue
		}
		b.WriteString(formatDeclOneLine(parsed.Fset, value.Decl))
	}
	for _, fn := range typ.Funcs {
		if !isVisibleName(fn.Name, opts) {
			continue
		}
		b.WriteString("\n")
		if opts.All {
			writeFunc(b, parsed.Package, parsed.Fset, fn, opts)
			continue
		}
		b.WriteString(formatFuncSig(parsed.Fset, fn))
	}
	for _, method := range typ.Methods {
		if !isVisibleName(method.Name, opts) {
			continue
		}
		b.WriteString("\n")
		if opts.All {
			writeFunc(b, parsed.Package, parsed.Fset, method, opts)
			continue
		}
		b.WriteString(formatFuncSig(parsed.Fset, method))
	}
}

func writeMethodDoc(b *strings.Builder, parsed *ParsedPackage, lookup Lookup, opts Options) bool {
	if lookup.Symbol == nil || lookup.Symbol.Member == nil {
		return false
	}
	symbol := lookup.Symbol.Name
	member := *lookup.Symbol.Member
	for _, typ := range parsed.Package.Types {
		if !matchDocName(symbol, typ.Name, opts) {
			continue
		}
		for _, method := range typ.Methods {
			if matchDocName(member, method.Name, opts) {
				writeFunc(b, parsed.Package, parsed.Fset, method, opts)
				return true
			}
		}
		if writeInterfaceMethodDoc(b, parsed, typ.Name, member, opts) {
			return true
		}
	}
	return false
}

func writeFieldDoc(b *strings.Builder, parsed *ParsedPackage, lookup Lookup, opts Options) bool {
	if lookup.Symbol == nil || lookup.Symbol.Member == nil {
		return false
	}
	symbol := lookup.Symbol.Name
	member := *lookup.Symbol.Member
	for _, file := range parsed.Files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !matchDocName(symbol, typeSpec.Name.Name, opts) {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				if writeStructFieldDoc(b, parsed, typeSpec.Name.Name, structType, member, opts) {
					return true
				}
			}
		}
	}
	return false
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

func writeValueSummaries(b *strings.Builder, fset *token.FileSet, values []*doc.Value, opts Options) {
	for _, value := range values {
		if !valueVisible(value, opts) {
			continue
		}
		b.WriteString("\n")
		b.WriteString(formatDeclOneLine(fset, value.Decl))
	}
}

func writeFuncSummaries(b *strings.Builder, fset *token.FileSet, funcs []*doc.Func, opts Options) {
	for _, fn := range funcs {
		if !isVisibleName(fn.Name, opts) {
			continue
		}
		b.WriteString("\n")
		b.WriteString(formatFuncSig(fset, fn))
	}
}

func writeTypeSummaries(b *strings.Builder, fset *token.FileSet, types []*doc.Type, opts Options) {
	for _, typ := range types {
		if !isVisibleName(typ.Name, opts) {
			continue
		}
		b.WriteString("\n")
		b.WriteString(formatDeclOneLine(fset, typ.Decl))
		writeFuncSummaries(b, fset, typ.Funcs, opts)
		writeFuncSummaries(b, fset, typ.Methods, opts)
	}
}

func writeValues(b *strings.Builder, pkg *doc.Package, fset *token.FileSet, values []*doc.Value, opts Options) {
	for _, value := range values {
		if !valueVisible(value, opts) {
			continue
		}
		b.WriteString("\n")
		writeValue(b, pkg, fset, value, opts)
	}
}

func writeValue(b *strings.Builder, pkg *doc.Package, fset *token.FileSet, value *doc.Value, opts Options) {
	b.WriteString(formatDecl(fset, value.Decl))
	if value.Doc != "" && !opts.Src {
		b.WriteString("\n")
		b.WriteString(indentDoc(wrapDoc(pkg, value.Doc)))
	}
}

func writeValueOnce(b *strings.Builder, pkg *doc.Package, fset *token.FileSet, value *doc.Value, opts Options, printed map[*ast.GenDecl]bool) {
	if printed[value.Decl] {
		return
	}
	writeValue(b, pkg, fset, value, opts)
	printed[value.Decl] = true
}

func writeType(b *strings.Builder, pkg *doc.Package, fset *token.FileSet, typ *doc.Type, opts Options) {
	b.WriteString(formatTypeDecl(fset, typ, opts))
	if typ.Doc != "" && !opts.Src {
		b.WriteString("\n")
		b.WriteString(indentDoc(wrapDoc(pkg, typ.Doc)))
	}
}

func writeFunc(b *strings.Builder, pkg *doc.Package, fset *token.FileSet, fn *doc.Func, opts Options) {
	if opts.Src {
		b.WriteString(formatDecl(fset, fn.Decl))
	} else {
		b.WriteString(formatFuncSig(fset, fn))
	}
	if fn.Doc != "" && !opts.Src {
		b.WriteString("\n")
		b.WriteString(indentDoc(wrapDoc(pkg, fn.Doc)))
	}
}

func formatTypeDecl(fset *token.FileSet, typ *doc.Type, opts Options) string {
	decl := singleTypeDecl(typ, opts)
	return formatDecl(fset, decl)
}

func singleTypeDecl(typ *doc.Type, opts Options) *ast.GenDecl {
	decl := *typ.Decl
	for _, spec := range typ.Decl.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok || typeSpec.Name.Name != typ.Name {
			continue
		}
		specCopy := *typeSpec
		if !opts.Unexported && !opts.Src {
			specCopy.Type = trimUnexportedTypeExpr(typeSpec.Type)
		}
		decl.Specs = []ast.Spec{&specCopy}
		return &decl
	}
	return typ.Decl
}

func trimUnexportedTypeExpr(expr ast.Expr) ast.Expr {
	switch typed := expr.(type) {
	case *ast.StructType:
		trimmed := *typed
		trimmed.Fields = trimUnexportedFieldList(typed.Fields, false)
		return &trimmed
	case *ast.InterfaceType:
		trimmed := *typed
		trimmed.Methods = trimUnexportedFieldList(typed.Methods, true)
		return &trimmed
	default:
		return expr
	}
}

func trimUnexportedFieldList(fields *ast.FieldList, isInterface bool) *ast.FieldList {
	if fields == nil {
		return nil
	}
	trimmed := false
	list := make([]*ast.Field, 0, len(fields.List))
	for _, field := range fields.List {
		if fieldVisible(field, isInterface) {
			list = append(list, field)
			continue
		}
		trimmed = true
	}
	if !trimmed {
		return fields
	}
	what := "fields"
	if isInterface {
		what = "methods"
	}
	return &ast.FieldList{
		Opening: fields.Opening,
		List:    append(list, unexportedMarkerField(fields, what)),
		Closing: fields.Closing,
	}
}

func fieldVisible(field *ast.Field, isInterface bool) bool {
	names := field.Names
	if len(names) == 0 {
		name, ok := embeddedFieldName(field.Type, isInterface)
		if !ok {
			return true
		}
		return token.IsExported(name)
	}
	for _, name := range names {
		if !token.IsExported(name.Name) {
			return false
		}
	}
	return true
}

func embeddedFieldName(expr ast.Expr, isInterface bool) (string, bool) {
	if star, ok := expr.(*ast.StarExpr); ok && !isInterface {
		expr = star.X
	}
	switch typed := expr.(type) {
	case *ast.Ident:
		if isInterface && (typed.Name == "error" || typed.Name == "comparable") {
			return "Error", true
		}
		return typed.Name, true
	case *ast.SelectorExpr:
		return typed.Sel.Name, true
	default:
		return "", false
	}
}

func unexportedMarkerField(fields *ast.FieldList, what string) *ast.Field {
	pos := fields.Closing - 1
	return &ast.Field{
		Type: &ast.Ident{
			Name:    "",
			NamePos: pos,
		},
		Comment: &ast.CommentGroup{
			List: []*ast.Comment{{
				Text: fmt.Sprintf("// Has unexported %s.\n", what),
			}},
		},
	}
}

func isVisibleName(name string, opts Options) bool {
	return opts.Unexported || token.IsExported(name)
}

func valueVisible(value *doc.Value, opts Options) bool {
	for _, name := range value.Names {
		if isVisibleName(name, opts) {
			return true
		}
	}
	return false
}

func valueMatches(value *doc.Value, symbol string, matches func(string, string) bool) bool {
	for _, name := range value.Names {
		if matches(name, symbol) {
			return true
		}
	}
	return false
}

func writeDocSeparator(b *strings.Builder, alreadyWrote bool) {
	if alreadyWrote {
		b.WriteString("\n\n")
	}
}

func writeInterfaceMethodDoc(b *strings.Builder, parsed *ParsedPackage, symbol, member string, opts Options) bool {
	for _, file := range parsed.Files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !matchDocName(symbol, typeSpec.Name.Name, opts) {
					continue
				}
				interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}
				var methods []*ast.Field
				for _, field := range interfaceType.Methods.List {
					if len(field.Names) == 0 {
						continue
					}
					if matchDocName(member, field.Names[0].Name, opts) {
						methods = append(methods, field)
					}
				}
				if len(methods) == 0 {
					continue
				}
				fmt.Fprintf(b, "type %s interface {\n", typeSpec.Name.Name)
				for _, method := range methods {
					writeCommentGroup(b, method.Doc, "\t")
					fmt.Fprintf(b, "\t%s\n", formatInterfaceMethod(parsed.Fset, method))
				}
				b.WriteString("}\n")
				return true
			}
		}
	}
	return false
}

func writeStructFieldDoc(b *strings.Builder, parsed *ParsedPackage, typeName string, structType *ast.StructType, member string, opts Options) bool {
	unmatched := false
	var matched []*ast.Field
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		for _, name := range field.Names {
			if matchDocName(member, name.Name, opts) {
				matched = append(matched, field)
				continue
			}
			unmatched = true
		}
	}
	if len(matched) == 0 {
		return false
	}
	fmt.Fprintf(b, "type %s struct {\n", typeName)
	for _, field := range matched {
		writeCommentGroup(b, field.Doc, "\t")
		for _, name := range field.Names {
			if matchDocName(member, name.Name, opts) {
				fmt.Fprintf(b, "\t%s %s", name.Name, formatExpr(parsed.Fset, field.Type))
				if field.Comment != nil && len(field.Comment.List) > 0 {
					fmt.Fprintf(b, "  %s", field.Comment.List[0].Text)
				}
				b.WriteString("\n")
			}
		}
	}
	if unmatched {
		b.WriteString("\n\t// ... other fields elided ...\n")
	}
	b.WriteString("}\n")
	return true
}

func interfaceMethodExists(parsed *ParsedPackage, symbol, member string, opts Options) bool {
	for _, file := range parsed.Files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !matchDocName(symbol, typeSpec.Name.Name, opts) {
					continue
				}
				interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}
				for _, field := range interfaceType.Methods.List {
					if len(field.Names) > 0 && matchDocName(member, field.Names[0].Name, opts) {
						return true
					}
				}
			}
		}
	}
	return false
}

func structFieldExists(parsed *ParsedPackage, symbol, member string, opts Options) bool {
	for _, file := range parsed.Files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !matchDocName(symbol, typeSpec.Name.Name, opts) {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, field := range structType.Fields.List {
					for _, name := range field.Names {
						if matchDocName(member, name.Name, opts) {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func formatInterfaceMethod(fset *token.FileSet, field *ast.Field) string {
	var b strings.Builder
	if len(field.Names) > 0 {
		b.WriteString(field.Names[0].Name)
	}
	signature := strings.TrimPrefix(formatExpr(fset, field.Type), "func")
	b.WriteString(signature)
	return b.String()
}

func formatExpr(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, expr); err != nil {
		return fmt.Sprintf("/* format error: %v */", err)
	}
	return buf.String()
}

func writeCommentGroup(b *strings.Builder, group *ast.CommentGroup, indent string) {
	if group == nil {
		return
	}
	for _, line := range strings.Split(strings.TrimRight(group.Text(), "\n"), "\n") {
		if line == "" {
			b.WriteString(indent)
			b.WriteString("//\n")
			continue
		}
		b.WriteString(indent)
		b.WriteString("// ")
		b.WriteString(line)
		b.WriteString("\n")
	}
}

func matchDocName(query string, name string, opts Options) bool {
	if !opts.Unexported && !token.IsExported(name) {
		return false
	}
	if opts.CaseSensitive {
		return query == name
	}
	for _, q := range query {
		n, width := utf8.DecodeRuneInString(name)
		if n == utf8.RuneError && width == 0 {
			return false
		}
		name = name[width:]
		if q == n {
			continue
		}
		if unicode.IsLower(q) && simpleFold(q) == simpleFold(n) {
			continue
		}
		return false
	}
	return name == ""
}

func simpleFold(r rune) rune {
	for {
		next := unicode.SimpleFold(r)
		if next <= r {
			return next
		}
		r = next
	}
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
