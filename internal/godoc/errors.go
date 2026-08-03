package godoc

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/mod/module"
)

var (
	// ErrPackageNotFound identifies a resolved package miss after every source fetcher declines the lookup.
	ErrPackageNotFound = errors.New("package not found")
	// ErrLookupNotFound identifies a symbol, method, or field miss after package candidates were exhausted.
	ErrLookupNotFound = errors.New("lookup not found")
)

// cachedPackageAbsentError records conclusive cache evidence that a package
// directory exists for a selected module version but contains no Go files.
// It is an internal resolver outcome, not a CLI error contract.
type cachedPackageAbsentError struct {
	Module  module.Version
	Package string
}

func (e *cachedPackageAbsentError) Error() string {
	return fmt.Sprintf("package %s has no Go files in cached module %s@%s", e.Package, e.Module.Path, e.Module.Version)
}

// sourceResolutionError marks an operational error returned by a source
// fetcher. It lets ambiguous lookups try another interpretation without
// deferring parsing or formatting failures after source was loaded.
type sourceResolutionError struct {
	err error
}

func (e *sourceResolutionError) Error() string {
	return e.err.Error()
}

func (e *sourceResolutionError) Unwrap() error {
	return e.err
}

// PackageNotFoundError names the package that no source fetcher could load.
type PackageNotFoundError struct {
	Package string
}

func (e PackageNotFoundError) Error() string {
	if e.Package == "" {
		return ErrPackageNotFound.Error()
	}
	return fmt.Sprintf("package %s not found", e.Package)
}

func (e PackageNotFoundError) Unwrap() error {
	return ErrPackageNotFound
}

// LookupNotFoundError describes a failed lookup after one or more package candidates were checked.
type LookupNotFoundError struct {
	Symbol   *SymbolLookup
	Query    string
	Attempts []string
}

func (e LookupNotFoundError) Error() string {
	var b strings.Builder
	switch {
	case e.Symbol != nil && e.Symbol.Member != nil:
		fmt.Fprintf(&b, "no method or field %s.%s", e.Symbol.Name, *e.Symbol.Member)
	case e.Symbol != nil:
		fmt.Fprintf(&b, "no symbol %s", e.Symbol.Name)
	case e.Query != "":
		fmt.Fprintf(&b, "no documentation found for %q", e.Query)
	default:
		b.WriteString(ErrLookupNotFound.Error())
	}
	attempts := uniqueStrings(e.Attempts)
	if len(attempts) > 0 {
		fmt.Fprintf(&b, " in packages %s", strings.Join(attempts, ", "))
	}
	return b.String()
}

func (e LookupNotFoundError) Unwrap() error {
	return ErrLookupNotFound
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
