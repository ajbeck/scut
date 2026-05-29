//go:build mage

// Build environment configuration and shared helpers.
package main

//mage:multiline

import (
	"fmt"
	"os"
	"strings"
	"time"

	versionmeta "github.com/ajbeck/scut/internal/version"
)

const (
	binaryName = "scut"
	buildDir   = "bin"
	mainPkg    = "./cmd/scut"
	versionPkg = "github.com/ajbeck/scut/internal/version"
)

var platforms = []struct {
	GOOS   string
	GOARCH string
}{
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
}

func init() {
	os.Setenv("GOEXPERIMENT", "jsonv2")
}

func version() string {
	if v := strings.TrimSpace(os.Getenv("RELEASE_VERSION")); v != "" {
		return v
	}
	return versionmeta.Version
}

func buildMetadata() string {
	if m := os.Getenv("BUILD_METADATA"); m != "" {
		return m
	}
	return "local:" + time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func ldflags() string {
	flags := "-X " + versionPkg + ".Version=" + version() + " -X " + versionPkg + ".BuildMetadata=" + buildMetadata()
	if strings.EqualFold(os.Getenv("RELEASE"), "true") {
		flags = "-s -w " + flags
	}
	return flags
}

func binaryPath(goos, goarch string) string {
	if goos == "" || goarch == "" {
		return binaryName
	}
	return fmt.Sprintf("%s-%s-%s", binaryName, goos, goarch)
}
