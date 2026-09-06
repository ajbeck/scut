package godoc

import (
	"bytes"
	"errors"
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// SourceBuildContext selects the Go files visible to documentation lookup.
type SourceBuildContext struct {
	Context build.Context
	Err     error
}

func loadSourceBuildContext() SourceBuildContext {
	return sourceBuildContextFrom(newGoEnvLoader(), build.Default)
}

func sourceBuildContextFrom(loader goEnvLoader, base build.Context) SourceBuildContext {
	values := loader.loadFiles()
	context := base
	context.GOOS = loader.value(values, "GOOS", context.GOOS)
	context.GOARCH = loader.value(values, "GOARCH", context.GOARCH)

	cgo := loader.value(values, "CGO_ENABLED", "")
	switch cgo {
	case "1":
		context.CgoEnabled = true
	case "0":
		context.CgoEnabled = false
	case "":
		if context.GOOS != runtime.GOOS || context.GOARCH != runtime.GOARCH {
			context.CgoEnabled = false
		}
	default:
		return SourceBuildContext{Context: context, Err: fmt.Errorf("invalid CGO_ENABLED value %q", cgo)}
	}

	tags, err := buildTagsFromGOFLAGS(loader.value(values, "GOFLAGS", ""))
	if err != nil {
		return SourceBuildContext{Context: context, Err: err}
	}
	context.BuildTags = tags
	return SourceBuildContext{Context: context}
}

// Filter returns only files that the active Go build context would compile.
func (c SourceBuildContext) Filter(files []SourceFile) ([]SourceFile, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	context := c.Context
	if context.GOOS == "" || context.GOARCH == "" {
		context = build.Default
	}
	byName := make(map[string][]byte, len(files))
	for _, file := range files {
		byName[filepath.Base(file.Name)] = file.Data
	}
	context.OpenFile = func(name string) (io.ReadCloser, error) {
		data, ok := byName[filepath.Base(name)]
		if !ok {
			return nil, os.ErrNotExist
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	}

	selected := make([]SourceFile, 0, len(files))
	for _, file := range files {
		name := filepath.Base(file.Name)
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		match, err := context.MatchFile(".", name)
		if err != nil {
			return nil, fmt.Errorf("matching build constraints for %s: %w", file.Name, err)
		}
		if !match || !context.CgoEnabled && importsC(file) {
			continue
		}
		selected = append(selected, file)
	}
	if len(selected) == 0 {
		return nil, ErrNoGoFiles
	}
	return selected, nil
}

func importsC(file SourceFile) bool {
	parsed, err := parser.ParseFile(token.NewFileSet(), file.Name, file.Data, parser.ImportsOnly)
	if err != nil {
		return false
	}
	for _, spec := range parsed.Imports {
		if spec.Path.Value == `"C"` {
			return true
		}
	}
	return false
}

func buildTagsFromGOFLAGS(value string) ([]string, error) {
	flags, err := splitConfiguredFields(value)
	if err != nil {
		return nil, fmt.Errorf("parsing GOFLAGS: %w", err)
	}
	var tags []string
	for _, flag := range flags {
		if !strings.HasPrefix(flag, "-") || flag == "-" || flag == "--" ||
			strings.HasPrefix(flag, "---") || strings.HasPrefix(flag, "-=") || strings.HasPrefix(flag, "--=") {
			return nil, fmt.Errorf("parsing GOFLAGS: non-flag %q", flag)
		}
		name := strings.TrimPrefix(flag, "-")
		name = strings.TrimPrefix(name, "-")
		if name == "tags" {
			return nil, errors.New("parsing GOFLAGS: -tags requires an =value")
		}
		value, ok := strings.CutPrefix(name, "tags=")
		if !ok {
			continue
		}
		if strings.Contains(value, " ") || strings.Contains(value, "'") {
			tags, err = splitConfiguredFields(value)
			if err != nil {
				return nil, fmt.Errorf("parsing GOFLAGS -tags: %w", err)
			}
			continue
		}
		tags = tags[:0]
		for tag := range strings.SplitSeq(value, ",") {
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}
	return tags, nil
}

func splitConfiguredFields(value string) ([]string, error) {
	var fields []string
	for len(value) > 0 {
		value = strings.TrimLeft(value, " \t\n\r")
		if value == "" {
			break
		}
		if value[0] == '\'' || value[0] == '"' {
			quote := value[0]
			end := strings.IndexByte(value[1:], quote)
			if end < 0 {
				return nil, fmt.Errorf("unterminated %c string", quote)
			}
			fields = append(fields, value[1:end+1])
			value = value[end+2:]
			continue
		}
		end := strings.IndexAny(value, " \t\n\r")
		if end < 0 {
			fields = append(fields, value)
			break
		}
		fields = append(fields, value[:end])
		value = value[end:]
	}
	return fields, nil
}
