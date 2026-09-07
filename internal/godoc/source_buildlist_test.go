package godoc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/spf13/afero"
)

func TestBuildListFetcherSelectsExternalModuleWithoutReadingItsDirectory(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeTestFile(t, fs, "/workspace/cache/lib@v1.2.3/pkg/pkg.go", []byte("package pkg\n"))
	runner := &fakeBuildListRunner{output: []byte(`
{"Path":"example.com/lib","Version":"v1.2.3","Dir":"/workspace/cache/lib@v1.2.3"}
	`)}
	fetcher := &BuildListFetcher{FS: fs, WorkDir: "/workspace/project", Runner: runner}

	selected, ok := fetcher.Select(context.Background(), "example.com/lib/pkg", Options{})
	if !ok {
		t.Fatal("Select() ok = false, want true")
	}
	if got, want := selected.Module.Path, "example.com/lib"; got != want {
		t.Fatalf("Module.Path = %q, want %q", got, want)
	}
	if got, want := selected.Module.Version, "v1.2.3"; got != want {
		t.Fatalf("Module.Version = %q, want %q", got, want)
	}
	if selected.Dir != "" {
		t.Fatalf("Dir = %q, want empty external source directory", selected.Dir)
	}
	if _, err := fetcher.Fetch(context.Background(), "example.com/lib/pkg", Options{}); !errors.Is(err, ErrSourceNotApplicable) {
		t.Fatalf("Fetch() error = %v, want ErrSourceNotApplicable", err)
	}
	if got, want := runner.calls, 1; got != want {
		t.Fatalf("Run() calls = %d, want %d", got, want)
	}

	_, _ = fetcher.Select(context.Background(), "example.com/lib/pkg", Options{})
	if got, want := runner.calls, 1; got != want {
		t.Fatalf("Run() calls = %d after second selection, want %d", got, want)
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

func TestBuildListFetcherSelectsVersionedReplacementArchive(t *testing.T) {
	fetcher := &BuildListFetcher{
		FS: afero.NewMemMapFs(),
		Runner: &fakeBuildListRunner{output: []byte(`
{"Path":"example.com/lib","Version":"v1.2.3","Replace":{"Path":"example.com/fork","Version":"v1.4.0","Dir":"/workspace/cache/fork@v1.4.0"}}
`)},
	}

	selected, ok := fetcher.Select(context.Background(), "example.com/lib/pkg", Options{})
	if !ok {
		t.Fatal("Select() ok = false, want true")
	}
	if got, want := selected.Source.Path, "example.com/fork"; got != want {
		t.Fatalf("Source.Path = %q, want %q", got, want)
	}
	if got, want := selected.Source.Version, "v1.4.0"; got != want {
		t.Fatalf("Source.Version = %q, want %q", got, want)
	}
	if selected.Dir != "" {
		t.Fatalf("Dir = %q, want empty versioned replacement directory", selected.Dir)
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
