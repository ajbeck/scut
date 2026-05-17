package godoc

import (
	"context"
	"errors"
	"fmt"
	"go/doc"
	"go/token"
	"path/filepath"
	"strings"
)

// SymbolLookup identifies a top-level symbol and optional method or field.
type SymbolLookup struct {
	Name   string
	Member *string
}

// Lookup is the normalized documentation target.
type Lookup struct {
	Package  string
	UserPath string
	Symbol   *SymbolLookup
}

// LookupCandidate is one package/symbol interpretation of raw doc arguments.
type LookupCandidate struct {
	Package              string
	UserPath             string
	Symbol               *SymbolLookup
	Kind                 LookupCandidateKind
	ContinueOnSymbolMiss bool
}

// LookupCandidateKind explains why a candidate was generated.
type LookupCandidateKind int

const (
	LookupCurrentPackage LookupCandidateKind = iota
	LookupFullPackage
	LookupSuffix
	LookupLocalPath
)

// ResolvedLookup is a lookup bound to fetched source.
type ResolvedLookup struct {
	Source   PackageSource
	Lookup   Lookup
	Attempts []string
}

// CurrentPackage describes the working package for no-argument and local symbol lookups.
type CurrentPackage struct {
	WorkDir    string
	ModuleDir  string
	ModulePath string
}

// LookupResolver resolves official go doc argument forms to package source.
type LookupResolver struct {
	Resolver     Resolver
	PackageIndex PackageIndex
	Current      CurrentPackage
}

func (r LookupResolver) Resolve(ctx context.Context, opts Options) (ResolvedLookup, error) {
	candidates, err := lookupCandidates(lookupArgs(opts))
	if err != nil {
		return ResolvedLookup{}, err
	}

	var attempts []string
	var missedSymbol *SymbolLookup
	for _, candidate := range candidates {
		if candidate.Symbol != nil {
			missedSymbol = candidate.Symbol
		}
		if candidate.Kind == LookupSuffix {
			resolved, ok, err := r.resolveSuffixCandidate(ctx, candidate, opts, attempts)
			if err != nil {
				return ResolvedLookup{}, err
			}
			if ok {
				return resolved, nil
			}
			attempts = resolved.Attempts
			continue
		}

		pkg := r.candidatePackage(candidate)
		attempts = append(attempts, pkg)
		resolved, ok, err := r.resolvePackageCandidate(ctx, candidate, pkg, opts, attempts)
		if err != nil {
			return ResolvedLookup{}, err
		}
		if ok {
			return resolved, nil
		}
		attempts = resolved.Attempts
	}

	if missedSymbol == nil {
		return ResolvedLookup{}, PackageNotFoundError{Package: strings.Join(lookupArgs(opts), " ")}
	}
	return ResolvedLookup{}, LookupNotFoundError{
		Symbol:   missedSymbol,
		Query:    strings.Join(lookupArgs(opts), " "),
		Attempts: attempts,
	}
}

func (r LookupResolver) resolveSuffixCandidate(ctx context.Context, candidate LookupCandidate, opts Options, attempts []string) (ResolvedLookup, bool, error) {
	index := r.PackageIndex
	if index == nil {
		index = emptyPackageIndex{}
	}
	matches, err := index.MatchSuffix(candidate.Package)
	if err != nil {
		return ResolvedLookup{Attempts: attempts}, false, err
	}
	for _, match := range matches {
		attempts = append(attempts, match.ImportPath)
		resolved, ok, err := r.resolvePackageCandidate(ctx, candidate, match.ImportPath, opts, attempts)
		if err != nil {
			return ResolvedLookup{}, false, err
		}
		if ok {
			return resolved, true, nil
		}
		attempts = resolved.Attempts
	}
	return ResolvedLookup{Attempts: attempts}, false, nil
}

func (r LookupResolver) resolvePackageCandidate(ctx context.Context, candidate LookupCandidate, pkg string, opts Options, attempts []string) (ResolvedLookup, bool, error) {
	source, err := r.Resolver.Fetch(ctx, pkg, opts)
	if err != nil {
		if errors.Is(err, ErrPackageNotFound) || errors.Is(err, ErrSourceNotApplicable) {
			return ResolvedLookup{Attempts: attempts}, false, nil
		}
		return ResolvedLookup{}, false, err
	}

	lookup := Lookup{
		Package:  source.ImportPath,
		UserPath: candidate.UserPath,
		Symbol:   candidate.Symbol,
	}
	parsed, err := ParsePackage(source.ImportPath, source.Files, lookupParseMode(opts))
	if err != nil {
		return ResolvedLookup{}, false, err
	}
	if lookup.Symbol == nil || LookupExists(parsed, lookup, opts) {
		return ResolvedLookup{Source: source, Lookup: lookup, Attempts: attempts}, true, nil
	}
	if candidate.ContinueOnSymbolMiss {
		return ResolvedLookup{Attempts: attempts}, false, nil
	}
	return ResolvedLookup{}, false, LookupNotFoundError{
		Symbol:   lookup.Symbol,
		Query:    strings.Join(lookupArgs(opts), " "),
		Attempts: attempts,
	}
}

func (r LookupResolver) candidatePackage(candidate LookupCandidate) string {
	if candidate.Kind != LookupCurrentPackage {
		return candidate.Package
	}
	if r.Current.ModuleDir == "" || r.Current.ModulePath == "" || r.Current.WorkDir == "" {
		return "."
	}
	rel, err := filepath.Rel(r.Current.ModuleDir, r.Current.WorkDir)
	if err != nil || !filepath.IsLocal(rel) {
		return "."
	}
	if rel == "." {
		return "."
	}
	return "./" + filepath.ToSlash(rel)
}

func lookupCandidates(args []string) ([]LookupCandidate, error) {
	switch len(args) {
	case 0:
		return []LookupCandidate{{Kind: LookupCurrentPackage}}, nil
	case 1:
		return oneArgLookupCandidates(args[0])
	case 2:
		return twoArgLookupCandidates(args[0], args[1])
	default:
		return nil, fmt.Errorf("usage: go doc accepts zero, one, or two arguments")
	}
}

func oneArgLookupCandidates(arg string) ([]LookupCandidate, error) {
	if isLocalPackageArg(arg) {
		return []LookupCandidate{{Package: arg, UserPath: arg, Kind: LookupLocalPath}}, nil
	}
	if !strings.Contains(arg, "/") && token.IsExported(arg) {
		symbol, err := parseSymbolSpec(arg)
		if err != nil {
			return nil, err
		}
		return []LookupCandidate{{Symbol: symbol, Kind: LookupCurrentPackage}}, nil
	}

	candidates := []LookupCandidate{{Package: arg, UserPath: arg, Kind: LookupFullPackage}}
	slash := strings.LastIndex(arg, "/")
	for start := slash + 1; start < len(arg); {
		period := strings.Index(arg[start:], ".")
		if period < 0 {
			break
		}
		period += start
		symbol, err := parseSymbolSpec(arg[period+1:])
		if err != nil {
			return nil, err
		}
		pkg := arg[:period]
		candidates = append(candidates, LookupCandidate{
			Package:              pkg,
			UserPath:             pkg,
			Symbol:               symbol,
			Kind:                 candidateKindForPackage(pkg),
			ContinueOnSymbolMiss: !strings.Contains(pkg, "/"),
		})
		start = period + 1
	}
	return candidates, nil
}

func twoArgLookupCandidates(pkg, rawSymbol string) ([]LookupCandidate, error) {
	symbol, err := parseSymbolSpec(rawSymbol)
	if err != nil {
		return nil, err
	}
	kind := candidateKindForPackage(pkg)
	if isLocalPackageArg(pkg) {
		kind = LookupLocalPath
	}
	candidate := LookupCandidate{
		Package:              pkg,
		UserPath:             pkg,
		Symbol:               symbol,
		Kind:                 kind,
		ContinueOnSymbolMiss: kind == LookupSuffix,
	}
	return []LookupCandidate{candidate}, nil
}

func parseSymbolSpec(raw string) (*SymbolLookup, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ".")
	switch len(parts) {
	case 1:
		return &SymbolLookup{Name: parts[0]}, nil
	case 2:
		return &SymbolLookup{Name: parts[0], Member: new(parts[1])}, nil
	default:
		return nil, fmt.Errorf("too many periods in symbol specification")
	}
}

func isLocalPackageArg(arg string) bool {
	return arg == "." ||
		arg == ".." ||
		strings.HasPrefix(arg, "./") ||
		strings.HasPrefix(arg, "../") ||
		strings.HasPrefix(arg, `.\`) ||
		strings.HasPrefix(arg, `..\`) ||
		filepath.IsAbs(arg)
}

func candidateKindForPackage(pkg string) LookupCandidateKind {
	if strings.Contains(pkg, "/") || filepath.IsAbs(pkg) {
		return LookupFullPackage
	}
	return LookupSuffix
}

func lookupArgs(opts Options) []string {
	if len(opts.Args) > 0 || opts.Package == "" {
		return opts.Args
	}
	if opts.Symbol != "" {
		return []string{opts.Package, opts.Symbol}
	}
	return []string{opts.Package}
}

func lookupParseMode(opts Options) doc.Mode {
	mode := doc.AllDecls
	if opts.Src {
		mode |= doc.PreserveAST
	}
	return mode
}
