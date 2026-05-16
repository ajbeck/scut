package version

import (
	"bytes"
	"testing"

	versionmeta "github.com/ajbeck/scut/internal/version"
)

func TestRunPrintsVersionString(t *testing.T) {
	orig, origMeta := versionmeta.Version, versionmeta.BuildMetadata
	t.Cleanup(func() {
		versionmeta.Version = orig
		versionmeta.BuildMetadata = origMeta
	})

	versionmeta.Version = "v1.2.3"
	versionmeta.BuildMetadata = "abc123"

	var stdout bytes.Buffer
	cmd := &Cmd{}
	if err := cmd.Run(&stdout); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got, want := stdout.String(), "v1.2.3+abc123\n"; got != want {
		t.Errorf("Run() wrote %q, want %q", got, want)
	}
}
