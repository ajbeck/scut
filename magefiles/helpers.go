//go:build mage

// Build environment configuration and shared helpers.
package main

//mage:multiline

import (
	"os"
	"time"
)

const (
	binaryName = "botctrl"
	buildDir   = "bin"
	mainPkg    = "./cmd/botctrl"
	versionPkg = "github.com/ajbeck/botctrl/internal/version"
)

func init() {
	os.Setenv("GOEXPERIMENT", "jsonv2")
}

func buildMetadata() string {
	if m := os.Getenv("BUILD_METADATA"); m != "" {
		return m
	}
	return "local:" + time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func ldflags() string {
	return "-X " + versionPkg + ".BuildMetadata=" + buildMetadata()
}
