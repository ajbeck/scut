//go:build goexperiment.jsonv2

package godoc

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/afero"
)

func TestBuildListFetcherLoadsSelectedModule(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeTestFile(t, fs, "/workspace/cache/lib@v1.2.3/pkg/pkg.go", []byte("package pkg\n"))
	runner := &fakeBuildListRunner{output: []byte(`
{"Path":"example.com/lib","Version":"v1.2.3","Dir":"/workspace/cache/lib@v1.2.3"}
`)}
	fetcher := &BuildListFetcher{FS: fs, WorkDir: "/workspace/project", Runner: runner}

	source, err := fetcher.Fetch(context.Background(), "example.com/lib/pkg", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got, want := source.Dir, filepath.Join("/workspace/cache/lib@v1.2.3", "pkg"); got != want {
		t.Fatalf("Dir = %q, want %q", got, want)
	}
	if got, want := source.Module.Path, "example.com/lib"; got != want {
		t.Fatalf("Module.Path = %q, want %q", got, want)
	}
	if got, want := source.Version, "v1.2.3"; got != want {
		t.Fatalf("Version = %q, want %q", got, want)
	}
	if got, want := runner.calls, 1; got != want {
		t.Fatalf("Run() calls = %d, want %d", got, want)
	}

	_, err = fetcher.Fetch(context.Background(), "example.com/lib/pkg", Options{})
	if err != nil {
		t.Fatalf("second Fetch() error = %v", err)
	}
	if got, want := runner.calls, 1; got != want {
		t.Fatalf("Run() calls = %d after second fetch, want %d", got, want)
	}
}

func TestBuildListFetcherUsesReplacementDirectory(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeTestFile(t, fs, "/workspace/replacement/pkg/pkg.go", []byte("package pkg\n"))
	fetcher := &BuildListFetcher{
		FS: fs,
		Runner: &fakeBuildListRunner{output: []byte(`
{"Path":"example.com/lib","Version":"v1.2.3","Dir":"/workspace/cache/lib@v1.2.3","Replace":{"Path":"../replacement","Dir":"/workspace/replacement"}}
`)},
	}

	source, err := fetcher.Fetch(context.Background(), "example.com/lib/pkg", Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got, want := source.Dir, "/workspace/replacement/pkg"; got != want {
		t.Fatalf("Dir = %q, want %q", got, want)
	}
}

func TestBuildListFetcherFallsBackWhenDiscoveryFails(t *testing.T) {
	runner := &fakeBuildListRunner{err: errors.New("go list failed")}
	fetcher := &BuildListFetcher{FS: afero.NewMemMapFs(), Runner: runner}

	for range 2 {
		_, err := fetcher.Fetch(context.Background(), "example.com/lib/pkg", Options{})
		if !errors.Is(err, ErrSourceNotApplicable) {
			t.Fatalf("Fetch() error = %v, want ErrSourceNotApplicable", err)
		}
	}
	if got, want := runner.calls, 1; got != want {
		t.Fatalf("Run() calls = %d, want %d", got, want)
	}
}

func TestBuildListFetcherFallsBackForMalformedOutput(t *testing.T) {
	fetcher := &BuildListFetcher{
		FS:     afero.NewMemMapFs(),
		Runner: &fakeBuildListRunner{output: []byte(`{"Path":`)},
	}

	_, err := fetcher.Fetch(context.Background(), "example.com/lib/pkg", Options{})
	if !errors.Is(err, ErrSourceNotApplicable) {
		t.Fatalf("Fetch() error = %v, want ErrSourceNotApplicable", err)
	}
}

func TestBuildListFetcherSkipsDifferentExplicitVersion(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeTestFile(t, fs, "/workspace/cache/lib@v1.2.3/pkg/pkg.go", []byte("package pkg\n"))
	fetcher := &BuildListFetcher{
		FS: fs,
		Runner: &fakeBuildListRunner{output: []byte(`
{"Path":"example.com/lib","Version":"v1.2.3","Dir":"/workspace/cache/lib@v1.2.3"}
`)},
	}

	_, err := fetcher.Fetch(context.Background(), "example.com/lib/pkg", Options{Version: "v1.2.4"})
	if !errors.Is(err, ErrSourceNotApplicable) {
		t.Fatalf("Fetch() error = %v, want ErrSourceNotApplicable", err)
	}
}

func TestBuildListFetcherUsesBoundedContext(t *testing.T) {
	runner := &fakeBuildListRunner{checkContext: func(ctx context.Context) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("Run() context has no deadline")
			return
		}
		if remaining := time.Until(deadline); remaining <= 0 || remaining > 2*time.Second {
			t.Fatalf("Run() deadline remaining = %s, want within 2 seconds", remaining)
		}
	}}
	fetcher := &BuildListFetcher{FS: afero.NewMemMapFs(), Runner: runner, Timeout: 2 * time.Second}

	_, err := fetcher.Fetch(context.Background(), "example.com/lib/pkg", Options{})
	if !errors.Is(err, ErrSourceNotApplicable) {
		t.Fatalf("Fetch() error = %v, want ErrSourceNotApplicable", err)
	}
}

type fakeBuildListRunner struct {
	output       []byte
	err          error
	calls        int
	checkContext func(context.Context)
}

func (r *fakeBuildListRunner) Run(ctx context.Context, _ string) ([]byte, error) {
	r.calls++
	if r.checkContext != nil {
		r.checkContext(ctx)
	}
	return r.output, r.err
}

var _ BuildListRunner = (*fakeBuildListRunner)(nil)
