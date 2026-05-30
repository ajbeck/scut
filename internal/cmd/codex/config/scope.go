//go:build goexperiment.jsonv2

package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func projectHooksPath(cwd string) string {
	return filepath.Join(cwd, ".codex", "hooks.json")
}

func userHooksPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving user home directory: %w", err)
	}
	return filepath.Join(home, ".codex", "hooks.json"), nil
}

func resolveScope(scope string) (string, error) {
	switch scope {
	case "project":
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolving cwd: %w", err)
		}
		return projectHooksPath(cwd), nil
	case "user":
		return userHooksPath()
	default:
		return "", fmt.Errorf("unknown scope %q", scope)
	}
}

func resolveScopePaths(scope string) ([]string, error) {
	switch scope {
	case "project":
		path, err := resolveScope("project")
		if err != nil {
			return nil, err
		}
		return []string{path}, nil
	case "user":
		path, err := resolveScope("user")
		if err != nil {
			return nil, err
		}
		return []string{path}, nil
	case "both":
		project, err := resolveScope("project")
		if err != nil {
			return nil, err
		}
		user, err := resolveScope("user")
		if err != nil {
			return nil, err
		}
		return []string{project, user}, nil
	default:
		return nil, fmt.Errorf("unknown scope %q", scope)
	}
}
