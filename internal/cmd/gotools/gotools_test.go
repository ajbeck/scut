package gotools

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/spf13/afero"

	"github.com/ajbeck/scut/internal/godoc"
)

func TestDocCmdAllowsNoArguments(t *testing.T) {
	fake := &fakeDocClient{output: "current package docs"}
	restore := replaceDocClientFactory(t, fake)
	defer restore()

	cmd := &docCmd{}
	var stdout bytes.Buffer

	if err := cmd.Run(&stdout, afero.NewMemMapFs()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := stdout.String(), "current package docs\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if len(fake.opts.Args) != 0 {
		t.Fatalf("options Args = %#v, want empty", fake.opts.Args)
	}
}

func TestDocCmdMapsOptionsAndWritesOneTrailingNewline(t *testing.T) {
	fake := &fakeDocClient{output: "doc output\n\n"}
	restore := replaceDocClientFactory(t, fake)
	defer restore()

	cmd := &docCmd{
		Args:    []string{"encoding/json", "Marshal"},
		All:     true,
		Short:   true,
		Src:     true,
		U:       true,
		C:       true,
		Cmd:     true,
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
		Args:          []string{"encoding/json", "Marshal"},
		Version:       "v1.2.3",
		All:           true,
		Short:         true,
		Src:           true,
		Unexported:    true,
		CaseSensitive: true,
		Cmd:           true,
	}
	if !reflect.DeepEqual(fake.opts, want) {
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
