package main

import (
	"bytes"
	"io"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"

	versionmeta "github.com/ajbeck/scut/internal/version"
)

func TestVersionCommand(t *testing.T) {
	orig, origMeta := versionmeta.Version, versionmeta.BuildMetadata
	t.Cleanup(func() {
		versionmeta.Version = orig
		versionmeta.BuildMetadata = origMeta
	})

	versionmeta.Version = "v2.3.4"
	versionmeta.BuildMetadata = "def456"

	var c cli
	var stdout bytes.Buffer
	parser := kong.Must(&c,
		kong.Name("scut"),
		kong.Vars{"version": versionmeta.String()},
		kong.BindTo(&stdout, (*io.Writer)(nil)),
		kong.BindTo(afero.NewMemMapFs(), (*afero.Fs)(nil)),
	)

	ctx, err := parser.Parse([]string{"version"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := ctx.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got, want := stdout.String(), "v2.3.4+def456\n"; got != want {
		t.Errorf("version command wrote %q, want %q", got, want)
	}
}

func TestGotoolsDocCommandParses(t *testing.T) {
	var c cli
	var stdout bytes.Buffer
	parser := kong.Must(&c,
		kong.Name("scut"),
		kong.Vars{"version": versionmeta.String()},
		kong.BindTo(&stdout, (*io.Writer)(nil)),
		kong.BindTo(afero.NewMemMapFs(), (*afero.Fs)(nil)),
	)

	ctx, err := parser.Parse([]string{"gotools", "doc", "encoding/json"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got, want := ctx.Command(), "gotools doc <lookup>"; got != want {
		t.Errorf("Command() = %q, want %q", got, want)
	}
}
