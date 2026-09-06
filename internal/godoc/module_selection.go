package godoc

import (
	"context"
	"strings"

	"golang.org/x/mod/module"
)

// ModuleSelection identifies the logical module selected for an import and the
// module or local directory that supplies its source.
type ModuleSelection struct {
	Module module.Version
	Source module.Version
	Dir    string
}

// ModuleSelector resolves the module selected for a package import.
type ModuleSelector interface {
	Select(context.Context, string, Options) (ModuleSelection, bool)
}

// ModuleSelectorChain returns the first module selection available from its
// ordered selectors.
type ModuleSelectorChain []ModuleSelector

func (c ModuleSelectorChain) Select(ctx context.Context, pkg string, opts Options) (ModuleSelection, bool) {
	for _, selector := range c {
		if selector == nil {
			continue
		}
		if selected, ok := selector.Select(ctx, pkg, opts); ok {
			return selected, true
		}
	}
	return ModuleSelection{}, false
}

// DependencySelector resolves versions parsed directly from the active go.mod
// when complete build-list discovery is unavailable.
type DependencySelector map[string]module.Version

func (s DependencySelector) Select(_ context.Context, pkg string, opts Options) (ModuleSelection, bool) {
	var selected module.Version
	for modulePath, candidate := range s {
		if pkg != modulePath && !strings.HasPrefix(pkg, modulePath+"/") {
			continue
		}
		if len(modulePath) > len(selected.Path) {
			selected = candidate
		}
	}
	if selected.Path == "" || !selectionMatchesVersion(selected, opts.Version) {
		return ModuleSelection{}, false
	}
	return ModuleSelection{Module: selected, Source: selected}, true
}

func selectionMatchesVersion(mod module.Version, requested string) bool {
	return requested == "" || requested == "latest" || requested == mod.Version
}

func packageSubdir(pkg, modulePath string) string {
	if pkg == modulePath {
		return ""
	}
	return strings.TrimPrefix(pkg, modulePath+"/")
}
