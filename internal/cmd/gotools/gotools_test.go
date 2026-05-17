package gotools

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/ajbeck/scut/internal/godoc"
)

func TestDocCmdRequiresPackage(t *testing.T) {
	cmd := &docCmd{}
	var stdout bytes.Buffer

	err := cmd.Run(&stdout, afero.NewMemMapFs())
	if err == nil {
		t.Fatal("Run() error = nil, want package-required error")
	}
	if !strings.Contains(err.Error(), "package is required") {
		t.Fatalf("Run() error = %q, want package-required error", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}

func TestDocCmdMapsOptionsAndWritesOneTrailingNewline(t *testing.T) {
	fake := &fakeDocClient{output: "doc output\n\n"}
	restore := replaceDocClientFactory(t, fake)
	defer restore()

	cmd := &docCmd{
		Package: "encoding/json",
		Symbol:  "Marshal",
		All:     true,
		Short:   true,
		Src:     true,
		U:       true,
		C:       true,
		Version: "v1.2.3",
	}
	var stdout bytes.Buffer

	if err := cmd.Run(&stdout, afero.NewMemMapFs()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := stdout.String(), "doc output\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	want := godoc.Options{
		Package:       "encoding/json",
		Symbol:        "Marshal",
		Version:       "v1.2.3",
		All:           true,
		Short:         true,
		Src:           true,
		Unexported:    true,
		CaseSensitive: true,
	}
	if fake.opts != want {
		t.Fatalf("options = %#v, want %#v", fake.opts, want)
	}
}

type fakeDocClient struct {
	output string
	opts   godoc.Options
}

func (c *fakeDocClient) Doc(_ context.Context, opts godoc.Options) (string, error) {
	c.opts = opts
	return c.output, nil
}

func replaceDocClientFactory(t *testing.T, client *fakeDocClient) func() {
	t.Helper()
	orig := newDocClient
	newDocClient = func(afero.Fs) (docClient, error) {
		return client, nil
	}
	return func() {
		newDocClient = orig
	}
}
