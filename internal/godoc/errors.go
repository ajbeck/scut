package godoc

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrPackageNotFound identifies a resolved package miss after every source fetcher declines the lookup.
	ErrPackageNotFound = errors.New("package not found")
	// ErrLookupNotFound identifies a symbol, method, or field miss after package candidates were exhausted.
	ErrLookupNotFound = errors.New("lookup not found")
)

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
