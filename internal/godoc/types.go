// Package godoc parses, resolves, and formats Go documentation.
package godoc

import (
	"go/ast"
	"go/doc"
	"go/token"

	"golang.org/x/mod/module"
)

// Options describes a documentation lookup request.
type Options struct {
	Args          []string
	Package       string
	Symbol        string
	Version       string
	All           bool
	Short         bool
	Src           bool
	Unexported    bool
	CaseSensitive bool
	Cmd           bool
}

// SourceFile is a Go source file loaded from any supported source.
type SourceFile struct {
	Name string
	Data []byte
}

// PackageSource is the source material for one package.
type PackageSource struct {
	ImportPath string
	Dir        string
	Files      []SourceFile
	Module     module.Version
	Version    string
}

// ImportAliases maps local import aliases to import paths.
type ImportAliases map[string]string

// ParsedPackage is the documentation model plus metadata needed for formatting.
type ParsedPackage struct {
	Package *doc.Package
	Fset    *token.FileSet
	Files   []*ast.File
	Aliases ImportAliases
}
