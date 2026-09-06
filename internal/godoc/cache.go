package godoc

import (
	"path/filepath"

	"golang.org/x/mod/module"
)

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
