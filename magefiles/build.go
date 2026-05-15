//go:build mage

// Build targets for the scut CLI.
package main

import (
	"context"
	"os"
	"path/filepath"

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
