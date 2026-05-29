package version

import (
	"runtime/debug"
	"testing"
)

func TestString(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		metadata string
		want     string
	}{
		{
			name:    "valid semver no metadata",
			version: "v1.2.3",
			want:    "v1.2.3",
		},
		{
			name:     "valid semver with metadata",
			version:  "v1.2.3",
			metadata: "abc123",
			want:     "v1.2.3+abc123",
		},
		{
			name:    "prerelease canonicalized",
			version: "v1.2.3-rc.1",
			want:    "v1.2.3-rc.1",
		},
		{
			name:     "prerelease with metadata",
			version:  "v0.1.0-dev",
			metadata: "local:2026-03-31",
			want:     "v0.1.0-dev+local:2026-03-31",
		},
		{
			name:    "invalid version returned as-is",
			version: "broken",
			want:    "broken",
		},
		{
			name:     "invalid version with metadata",
			version:  "broken",
			metadata: "abc",
			want:     "broken+abc",
		},
		{
			name:    "default dev version",
			version: "v0.0.0-dev",
			want:    "v0.0.0-dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig, origMeta := Version, BuildMetadata
			t.Cleanup(func() { Version, BuildMetadata = orig, origMeta })

			Version = tt.version
			BuildMetadata = tt.metadata
			if got := String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStringUsesBuildInfoVersionForDefaultVersion(t *testing.T) {
	origVersion, origMeta, origRead := Version, BuildMetadata, readBuildInfo
	t.Cleanup(func() {
		Version = origVersion
		BuildMetadata = origMeta
		readBuildInfo = origRead
	})

	Version = defaultVersion
	BuildMetadata = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, true
	}

	if got := String(); got != "v1.2.3" {
		t.Errorf("String() = %q, want %q", got, "v1.2.3")
	}
}

func TestStringIgnoresDevelBuildInfoVersion(t *testing.T) {
	origVersion, origMeta, origRead := Version, BuildMetadata, readBuildInfo
	t.Cleanup(func() {
		Version = origVersion
		BuildMetadata = origMeta
		readBuildInfo = origRead
	})

	Version = defaultVersion
	BuildMetadata = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true
	}

	if got := String(); got != defaultVersion {
		t.Errorf("String() = %q, want %q", got, defaultVersion)
	}
}
