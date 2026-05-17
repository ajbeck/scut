package godoc

import (
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/mod/module"
)

// ResolvedModule identifies a concrete module version and package within it.
type ResolvedModule struct {
	Path        string
	Version     string
	PackagePath string
}

// WriteCache writes package source files into a Go module cache layout.
func WriteCache(fs afero.Fs, cacheDir string, resolved ResolvedModule, files []SourceFile) error {
	if cacheDir == "" {
		return nil
	}
	dir, err := resolvedModulePackageDir(cacheDir, resolved)
	if err != nil {
		return err
	}
	if err := fs.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, file := range files {
		name := filepath.Base(file.Name)
		if err := afero.WriteFile(fs, filepath.Join(dir, name), file.Data, 0644); err != nil {
			return err
		}
	}
	return nil
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
