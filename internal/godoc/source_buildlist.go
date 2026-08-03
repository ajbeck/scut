//go:build goexperiment.jsonv2

package godoc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"encoding/json/jsontext"
	json "encoding/json/v2"

	"github.com/spf13/afero"
	"golang.org/x/mod/module"
)

// BuildListRunner reads the active Go module build list for a working directory.
type BuildListRunner interface {
	Run(context.Context, string) ([]byte, error)
}

// GoBuildListRunner reads the build list with the Go command without modifying
// the active module or workspace.
type GoBuildListRunner struct{}

func (GoBuildListRunner) Run(ctx context.Context, workDir string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-mod=readonly", "-m", "-json", "all")
	cmd.Dir = workDir
	return cmd.Output()
}

// BuildListFetcher loads package source from the active Go build list. Discovery
// is lazy, bounded, and best-effort so it never blocks other source routes.
type BuildListFetcher struct {
	FS      afero.Fs
	WorkDir string
	Timeout time.Duration
	Runner  BuildListRunner

	once    sync.Once
	modules []buildListModule
	err     error
}

type buildListModule struct {
	Path    string
	Version string
	Dir     string
	Replace *buildListModule
}

func (f *BuildListFetcher) Fetch(ctx context.Context, pkg string, opts Options) (PackageSource, error) {
	modules, err := f.buildList(ctx)
	if err != nil {
		return PackageSource{}, ErrSourceNotApplicable
	}

	selected, ok := selectBuildListModule(modules, pkg)
	if !ok || (opts.Version != "" && opts.Version != "latest" && opts.Version != selected.Version) {
		return PackageSource{}, ErrSourceNotApplicable
	}

	dir, ok := buildListPackageDir(selected, pkg)
	if !ok {
		return PackageSource{}, ErrSourceNotApplicable
	}
	files, err := readGoFiles(f.FS, dir)
	if errors.Is(err, ErrNoGoFiles) {
		return PackageSource{}, ErrSourceNotApplicable
	}
	if err != nil {
		return PackageSource{}, err
	}

	resolved := module.Version{Path: selected.Path, Version: selected.Version}
	return PackageSource{
		ImportPath: pkg,
		Dir:        dir,
		Files:      files,
		Module:     resolved,
		Version:    resolved.Version,
	}, nil
}

func (f *BuildListFetcher) buildList(ctx context.Context) ([]buildListModule, error) {
	f.once.Do(func() {
		timeout := f.Timeout
		if timeout <= 0 {
			timeout = 2 * time.Second
		}
		queryCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		runner := f.Runner
		if runner == nil {
			runner = GoBuildListRunner{}
		}
		output, err := runner.Run(queryCtx, f.WorkDir)
		if err != nil {
			f.err = err
			return
		}
		f.modules, f.err = parseBuildList(output)
	})
	return f.modules, f.err
}

func parseBuildList(output []byte) ([]buildListModule, error) {
	decoder := jsontext.NewDecoder(bytes.NewReader(output))
	var modules []buildListModule
	for {
		var entry buildListModule
		err := json.UnmarshalDecode(decoder, &entry)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if entry.Path == "" || buildListModuleDir(entry) == "" {
			continue
		}
		modules = append(modules, entry)
	}
	return modules, nil
}

func selectBuildListModule(modules []buildListModule, pkg string) (buildListModule, bool) {
	var best buildListModule
	for _, candidate := range modules {
		if pkg != candidate.Path && !strings.HasPrefix(pkg, candidate.Path+"/") {
			continue
		}
		if len(candidate.Path) > len(best.Path) {
			best = candidate
		}
	}
	return best, best.Path != ""
}

func buildListPackageDir(selected buildListModule, pkg string) (string, bool) {
	root := buildListModuleDir(selected)
	if root == "" {
		return "", false
	}
	subdir := strings.TrimPrefix(pkg, selected.Path)
	subdir = strings.TrimPrefix(subdir, "/")
	dir := filepath.Clean(filepath.Join(root, filepath.FromSlash(subdir)))
	rel, err := filepath.Rel(root, dir)
	if err != nil || (rel != "." && !filepath.IsLocal(rel)) {
		return "", false
	}
	return dir, true
}

func buildListModuleDir(entry buildListModule) string {
	if entry.Replace != nil && entry.Replace.Dir != "" {
		return entry.Replace.Dir
	}
	return entry.Dir
}

var _ SourceFetcher = (*BuildListFetcher)(nil)
