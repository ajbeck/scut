package godoc

import (
	"errors"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"strconv"
)

var ErrNoParseableFiles = errors.New("no parseable Go files")

// ParsePackage parses source files into Go documentation.
func ParsePackage(importPath string, files []SourceFile, mode doc.Mode) (*ParsedPackage, error) {
	fset := token.NewFileSet()
	asts := make([]*ast.File, 0, len(files))
	aliases := ImportAliases{}

	for _, file := range files {
		parsed, err := parser.ParseFile(fset, file.Name, file.Data, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		asts = append(asts, parsed)
		collectAliases(aliases, parsed)
	}

	if len(asts) == 0 {
		return nil, ErrNoParseableFiles
	}

	pkg, err := doc.NewFromFiles(fset, asts, importPath, mode)
	if err != nil {
		return nil, fmt.Errorf("building package docs: %w", err)
	}

	return &ParsedPackage{
		Package: pkg,
		Fset:    fset,
		Files:   asts,
		Aliases: aliases,
	}, nil
}

func collectAliases(aliases ImportAliases, file *ast.File) {
	for _, spec := range file.Imports {
		if spec.Name == nil {
			continue
		}
		if spec.Name.Name == "." || spec.Name.Name == "_" {
			continue
		}
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		aliases[spec.Name.Name] = path
	}
}
