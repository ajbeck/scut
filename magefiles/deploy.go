//go:build mage

// Local deployment targets.
package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// LocalDeploy builds the binary and copies it to the given directory.
func LocalDeploy(ctx context.Context, dest string) error {
	mg.CtxDeps(ctx, Build)
	src := filepath.Join(buildDir, binaryName)
	dst := filepath.Join(dest, binaryName)
	if err := sh.Run("cp", src, dst); err != nil {
		return err
	}
	abs, _ := filepath.Abs(dst)
	fmt.Printf("deployed %s → %s\n", binaryName, abs)
	return nil
}
