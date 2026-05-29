// Package version provides build and version metadata for scut.
//
// [Version] and [BuildMetadata] are set at compile time via ldflags:
//
//	go build -ldflags "-X github.com/ajbeck/scut/internal/version.Version=v0.1.0 \
//	  -X github.com/ajbeck/scut/internal/version.BuildMetadata=abc123"
package version

import (
	"runtime/debug"

	"golang.org/x/mod/semver"
)

const defaultVersion = "v0.0.0-dev"

// Version is the semantic version of the application.
// Must include the "v" prefix (e.g., "v0.1.0", "v1.0.0-rc.1").
// Set at compile time via -ldflags.
var Version = defaultVersion

// BuildMetadata is additional build information such as a commit SHA.
// Set at compile time via -ldflags.
var BuildMetadata string

var readBuildInfo = debug.ReadBuildInfo

// String returns the full semver string.
// When [BuildMetadata] is set the result is "vMAJOR.MINOR.PATCH[-PRERELEASE]+BUILD".
// An invalid [Version] is returned as-is with metadata appended.
func String() string {
	v := baseVersion()
	if !semver.IsValid(v) {
		if BuildMetadata != "" {
			return v + "+" + BuildMetadata
		}
		return v
	}
	if BuildMetadata != "" {
		return semver.Canonical(v) + "+" + BuildMetadata
	}
	return semver.Canonical(v)
}

func baseVersion() string {
	if Version != defaultVersion {
		return Version
	}
	info, ok := readBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return Version
	}
	return info.Main.Version
}
