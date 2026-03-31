//go:build mage

// Test targets.
package main

import (
	"context"

	"github.com/magefile/mage/sh"
)

// Test runs all tests with the race detector enabled.
func Test(ctx context.Context) error {
	return sh.RunV("go", "test", "-race", "./...")
}
