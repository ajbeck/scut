package godoc

import (
	"path/filepath"
	"strings"

	"golang.org/x/mod/module"
)

// ResolvedModule identifies a concrete module version and package within it.
type ResolvedModule struct {
	Path        string
	Version     string
	PackagePath string
}

func resolvedModulePackageDir(cacheDir string, resolved ResolvedModule) (string, error) {
	moduleDir, err := moduleCachePath(cacheDir, resolved.Path, resolved.Version)
	if err != nil {
		return "", err
	}
	subdir := packageSubdir(resolved.PackagePath, resolved.Path)
	if subdir == "" {
		return moduleDir, nil
	}
	return filepath.Join(moduleDir, filepath.FromSlash(subdir)), nil
}

func moduleCachePath(cacheDir, modPath, version string) (string, error) {
	escaped, err := module.EscapePath(modPath)
	if err != nil {
		return "", err
	}
	escapedVersion, err := module.EscapeVersion(version)
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, filepath.FromSlash(escaped)+"@"+escapedVersion), nil
}

func packageSubdir(pkg, modPath string) string {
	if pkg == modPath {
		return ""
	}
	return strings.TrimPrefix(pkg, modPath+"/")
}
