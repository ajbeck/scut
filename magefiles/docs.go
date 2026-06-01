//go:build mage

// Documentation targets.
package main

import (
	"context"

	"github.com/magefile/mage/sh"
)

// Docs builds the Hugo documentation site into public/.
func Docs(ctx context.Context) error {
	return sh.Run("hugo", "--source", "docs", "--gc", "--minify")
}
