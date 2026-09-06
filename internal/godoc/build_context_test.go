package godoc

import (
	"errors"
	"go/build"
	"reflect"
	"runtime"
	"testing"
)

func TestSourceBuildContextFilter(t *testing.T) {
	files := []SourceFile{
		{Name: "common.go", Data: []byte("package widget\n\nconst Common = true\n")},
		{Name: "platform_linux.go", Data: []byte("package widget\n\nconst Linux = true\n")},
		{Name: "platform_windows.go", Data: []byte("package widget\n\nconst Windows = true\n")},
		{Name: "feature.go", Data: []byte("//go:build feature\n\npackage widget\n\nconst Feature = true\n")},
		{Name: "ignored.go", Data: []byte("//go:build ignore\n\npackage main\n")},
		{Name: "cgo.go", Data: []byte("package widget\n\nimport \"C\"\n")},
		{Name: "widget_test.go", Data: []byte("package widget_test\n")},
		{Name: "_generated.go", Data: []byte("package conflicting\n")},
		{Name: ".hidden.go", Data: []byte("package conflicting\n")},
	}
	context := build.Default
	context.GOOS = "linux"
	context.GOARCH = "amd64"
	context.CgoEnabled = false
	context.BuildTags = []string{"feature"}

	got, err := (SourceBuildContext{Context: context}).Filter(files)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}
	want := []SourceFile{files[0], files[1], files[3]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Filter() = %#v, want %#v", got, want)
	}
}

func TestSourceBuildContextFilterIncludesConfiguredIgnoreTag(t *testing.T) {
	context := build.Default
	context.BuildTags = []string{"ignore"}
	files := []SourceFile{{
		Name: "tool.go",
		Data: []byte("//go:build ignore\n\npackage main\n"),
	}}

	got, err := (SourceBuildContext{Context: context}).Filter(files)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}
	if !reflect.DeepEqual(got, files) {
		t.Fatalf("Filter() = %#v, want %#v", got, files)
	}
}

func TestSourceBuildContextFilterReturnsNoGoFiles(t *testing.T) {
	context := build.Default
	context.GOOS = "linux"
	files := []SourceFile{{
		Name: "platform_windows.go",
		Data: []byte("package widget\n"),
	}}

	_, err := (SourceBuildContext{Context: context}).Filter(files)
	if !errors.Is(err, ErrNoGoFiles) {
		t.Fatalf("Filter() error = %v, want ErrNoGoFiles", err)
	}
}

func TestSourceBuildContextFilterRejectsInvalidConstraint(t *testing.T) {
	files := []SourceFile{{
		Name: "broken.go",
		Data: []byte("//go:build linux &&\n\npackage widget\n"),
	}}

	_, err := (SourceBuildContext{Context: build.Default}).Filter(files)
	if err == nil {
		t.Fatal("Filter() error = nil, want invalid build constraint")
	}
}

func TestSourceBuildContextFromGoEnvironment(t *testing.T) {
	files := map[string][]byte{
		"/config/go/env": []byte("GOARCH=arm64\nGOFLAGS=-mod=readonly -tags=from_user,shared\n"),
		"/goroot/go.env": []byte("GOOS=plan9\nGOARCH=386\nCGO_ENABLED=1\nGOFLAGS=-tags=from_goroot\n"),
	}
	loader := goEnvLoader{
		Lookup: func(name string) (string, bool) {
			values := map[string]string{
				"GOENV": "/config/go/env",
				"GOOS":  "linux",
			}
			value, ok := values[name]
			return value, ok
		},
		ReadFile: func(name string) ([]byte, error) {
			data, ok := files[name]
			if !ok {
				return nil, errors.New("not found")
			}
			return data, nil
		},
		UserConfigDir: func() (string, error) { return "/unused", nil },
		GOROOT:        "/goroot",
	}
	base := build.Default
	base.ToolTags = []string{"tool-tag"}
	base.ReleaseTags = []string{"go1.26"}

	got := sourceBuildContextFrom(loader, base)
	if got.Err != nil {
		t.Fatalf("sourceBuildContextFrom() error = %v", got.Err)
	}
	if got.Context.GOOS != "linux" || got.Context.GOARCH != "arm64" {
		t.Fatalf("target = %s/%s, want linux/arm64", got.Context.GOOS, got.Context.GOARCH)
	}
	if !got.Context.CgoEnabled {
		t.Fatal("CgoEnabled = false, want GOROOT go.env value")
	}
	if want := []string{"from_user", "shared"}; !reflect.DeepEqual(got.Context.BuildTags, want) {
		t.Fatalf("BuildTags = %q, want %q", got.Context.BuildTags, want)
	}
	if !reflect.DeepEqual(got.Context.ToolTags, base.ToolTags) || !reflect.DeepEqual(got.Context.ReleaseTags, base.ReleaseTags) {
		t.Fatal("sourceBuildContextFrom() did not preserve tool and release tags")
	}
}

func TestSourceBuildContextDisablesCgoForCrossTargetByDefault(t *testing.T) {
	loader := goEnvLoader{
		Lookup: func(name string) (string, bool) {
			values := map[string]string{"GOOS": "not-" + runtime.GOOS}
			value, ok := values[name]
			return value, ok
		},
		ReadFile:      func(string) ([]byte, error) { return nil, errors.New("not found") },
		UserConfigDir: func() (string, error) { return "", errors.New("not found") },
	}

	got := sourceBuildContextFrom(loader, build.Default)
	if got.Err != nil {
		t.Fatalf("sourceBuildContextFrom() error = %v", got.Err)
	}
	if got.Context.CgoEnabled {
		t.Fatal("CgoEnabled = true for cross target without an explicit setting")
	}
}

func TestSourceBuildContextRejectsInvalidCgoSetting(t *testing.T) {
	loader := goEnvLoader{
		Lookup: func(name string) (string, bool) {
			if name == "CGO_ENABLED" {
				return "maybe", true
			}
			return "", false
		},
		ReadFile:      func(string) ([]byte, error) { return nil, errors.New("not found") },
		UserConfigDir: func() (string, error) { return "", errors.New("not found") },
	}

	got := sourceBuildContextFrom(loader, build.Default)
	if got.Err == nil {
		t.Fatal("sourceBuildContextFrom() error = nil, want invalid CGO_ENABLED")
	}
}

func TestBuildTagsFromGOFLAGS(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    []string
		wantErr bool
	}{
		{name: "unset"},
		{name: "comma list", value: "-mod=readonly -tags=one,two", want: []string{"one", "two"}},
		{name: "double dash", value: "--tags=feature", want: []string{"feature"}},
		{name: "last wins", value: "-tags=old -tags=new", want: []string{"new"}},
		{name: "quoted flag", value: `'--tags=one,two'`, want: []string{"one", "two"}},
		{name: "compat quoted value", value: `"-tags='one two'"`, want: []string{"one two"}},
		{name: "missing value", value: "-tags", wantErr: true},
		{name: "non flag", value: "-mod=readonly value", wantErr: true},
		{name: "triple dash", value: "---tags=feature", wantErr: true},
		{name: "unterminated quote", value: `"-tags=feature`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildTagsFromGOFLAGS(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("buildTagsFromGOFLAGS(%q) error = nil", tt.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildTagsFromGOFLAGS(%q) error = %v", tt.value, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("buildTagsFromGOFLAGS(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}
