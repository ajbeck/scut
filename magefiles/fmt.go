//go:build mage

// Formatting targets.
package main

import (
	"context"

	"github.com/magefile/mage/sh"
)

// Fmt runs gofmt -w on all Go source files.
func Fmt(ctx context.Context) error {
	return sh.RunV("gofmt", "-w", ".")
}
