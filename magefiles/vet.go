//go:build mage

// Static analysis targets.
package main

import (
	"context"

	"github.com/magefile/mage/sh"
)

// Vet runs go vet across all packages.
func Vet(ctx context.Context) error {
	return sh.Run("go", "vet", "./...")
}
