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
	selected, ok := f.Select(ctx, pkg, opts)
	if !ok || selected.Dir == "" {
		return PackageSource{}, ErrSourceNotApplicable
	}
	dir, ok := selectedPackageDir(selected, pkg)
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

	return PackageSource{
		ImportPath: pkg,
		Dir:        dir,
		Files:      files,
		Module:     selected.Module,
		Version:    selected.Module.Version,
	}, nil
}

// Select returns the active build-list module without treating an external
// module cache directory as package source.
func (f *BuildListFetcher) Select(ctx context.Context, pkg string, opts Options) (ModuleSelection, bool) {
	modules, err := f.buildList(ctx)
	if err != nil {
		return ModuleSelection{}, false
	}
	selected, ok := selectBuildListModule(modules, pkg)
	if !ok {
		return ModuleSelection{}, false
	}
	logical := module.Version{Path: selected.Path, Version: selected.Version}
	if !selectionMatchesVersion(logical, opts.Version) {
		return ModuleSelection{}, false
	}
	selection := ModuleSelection{Module: logical, Source: logical}
	if selected.Replace != nil {
		selection.Source = module.Version{Path: selected.Replace.Path, Version: selected.Replace.Version}
		if selected.Replace.Version == "" {
			selection.Dir = selected.Replace.Dir
		}
	} else if selected.Version == "" {
		selection.Dir = selected.Dir
	}
	return selection, true
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
		if entry.Path == "" {
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

func selectedPackageDir(selected ModuleSelection, pkg string) (string, bool) {
	root := selected.Dir
	if root == "" {
		return "", false
	}
	subdir := strings.TrimPrefix(pkg, selected.Module.Path)
	subdir = strings.TrimPrefix(subdir, "/")
	dir := filepath.Clean(filepath.Join(root, filepath.FromSlash(subdir)))
	rel, err := filepath.Rel(root, dir)
	if err != nil || (rel != "." && !filepath.IsLocal(rel)) {
		return "", false
	}
	return dir, true
}

var _ SourceFetcher = (*BuildListFetcher)(nil)
var _ ModuleSelector = (*BuildListFetcher)(nil)
