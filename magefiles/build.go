//go:build mage

// Build targets for the scut CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Default target when running mage with no arguments.
var Default = Build

// Build compiles the scut binary into bin/.
func Build(ctx context.Context) error {
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return err
	}
	return sh.Run("go", "build", "-ldflags", ldflags(), "-o", filepath.Join(buildDir, binaryName), mainPkg)
}

// BuildPlatform cross-compiles the scut binary for the given OS and architecture.
//
// Usage: mage buildPlatform darwin arm64
func BuildPlatform(ctx context.Context, goos, goarch string) error {
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return err
	}
	env := map[string]string{
		"GOOS":   goos,
		"GOARCH": goarch,
	}
	out := filepath.Join(buildDir, binaryPath(goos, goarch))
	return sh.RunWith(env, "go", "build", "-ldflags", ldflags(), "-o", out, mainPkg)
}

// BuildAll cross-compiles scut for all supported release platforms.
func BuildAll(ctx context.Context) error {
	var deps []any
	for _, p := range platforms {
		deps = append(deps, mg.F(BuildPlatform, p.GOOS, p.GOARCH))
	}
	mg.CtxDeps(ctx, deps...)
	return nil
}

// Version prints the base version used for builds.
func Version(ctx context.Context) {
	fmt.Println(version())
}
