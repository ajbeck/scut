package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"

	versionmeta "github.com/ajbeck/scut/internal/version"
)

func TestDryRunScriptInstallPlansVerifiedReplacement(t *testing.T) {
	resetUpdateTestState(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".local", "bin", "scut")
	executable = func() (string, error) { return path, nil }
	evalSymlinks = func(path string) (string, error) { return path, nil }
	versionmeta.Version = "v0.3.3"
	httpClient = fakeHTTPClient(map[string][]byte{
		"https://api.github.com/repos/ajbeck/scut/releases/latest": []byte(`{"tag_name":"v0.3.4"}`),
	})

	var stdout bytes.Buffer
	cmd := &Cmd{DryRun: true}
	if err := cmd.Run(&stdout, afero.NewOsFs(), slog.Default()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	out := stdout.String()
	assertContains(t, out, "current version: v0.3.3")
	assertContains(t, out, "target version: v0.3.4")
	assertContains(t, out, "install method: install-script")
	assertContains(t, out, "action: download scut-v0.3.4-")
	assertContains(t, out, "verify checksums.txt")
}

func TestDryRunHomebrewPrintsGuidance(t *testing.T) {
	resetUpdateTestState(t)
	executable = func() (string, error) { return "/opt/homebrew/bin/scut", nil }
	evalSymlinks = func(string) (string, error) { return "/opt/homebrew/Cellar/scut/0.3.3/bin/scut", nil }
	lookPath = func(name string) (string, error) { return "/opt/homebrew/bin/" + name, nil }
	versionmeta.Version = "v0.3.3"
	httpClient = fakeHTTPClient(map[string][]byte{
		"https://api.github.com/repos/ajbeck/scut/releases/latest": []byte(`{"tag_name":"v0.3.4"}`),
	})

	var stdout bytes.Buffer
	cmd := &Cmd{DryRun: true}
	if err := cmd.Run(&stdout, afero.NewOsFs(), slog.Default()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	out := stdout.String()
	assertContains(t, out, "install method: homebrew")
	assertContains(t, out, "brew update")
	assertContains(t, out, "brew upgrade ajbeck/tap/scut")
}

func TestSourceBuildDoesNotOverwrite(t *testing.T) {
	resetUpdateTestState(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "bin", "scut")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/ajbeck/scut\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	executable = func() (string, error) { return path, nil }
	evalSymlinks = func(path string) (string, error) { return path, nil }
	versionmeta.Version = "v0.3.3"

	var stdout bytes.Buffer
	cmd := &Cmd{Version: "v0.3.4"}
	if err := cmd.Run(&stdout, afero.NewOsFs(), slog.Default()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old" {
		t.Fatalf("source build was overwritten with %q", data)
	}
	assertContains(t, stdout.String(), "install method: source")
	assertContains(t, stdout.String(), "go install github.com/ajbeck/scut@latest")
}

func TestScriptInstallUpdatesExecutableAfterChecksumVerification(t *testing.T) {
	resetUpdateTestState(t)
	goos, goarch = "linux", "amd64"
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".local", "bin", "scut")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	executable = func() (string, error) { return path, nil }
	evalSymlinks = func(path string) (string, error) { return path, nil }
	versionmeta.Version = "v0.3.3"

	archive := releaseArchive(t, []byte("new"))
	sum := fmt.Sprintf("%x", sha256.Sum256(archive))
	httpClient = fakeHTTPClient(map[string][]byte{
		"https://github.com/ajbeck/scut/releases/download/v0.3.4/scut-v0.3.4-linux-amd64.tar.gz": archive,
		"https://github.com/ajbeck/scut/releases/download/v0.3.4/checksums.txt":                  []byte(sum + "  scut-v0.3.4-linux-amd64.tar.gz\n"),
	})

	var stdout bytes.Buffer
	cmd := &Cmd{Version: "v0.3.4"}
	if err := cmd.Run(&stdout, afero.NewOsFs(), slog.Default()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("updated executable = %q, want new", data)
	}
	assertContains(t, stdout.String(), "updated scut from v0.3.3 to v0.3.4")
}

func TestScriptInstallRejectsChecksumMismatch(t *testing.T) {
	resetUpdateTestState(t)
	goos, goarch = "linux", "amd64"
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".local", "bin", "scut")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	executable = func() (string, error) { return path, nil }
	evalSymlinks = func(path string) (string, error) { return path, nil }
	versionmeta.Version = "v0.3.3"

	archive := releaseArchive(t, []byte("new"))
	httpClient = fakeHTTPClient(map[string][]byte{
		"https://github.com/ajbeck/scut/releases/download/v0.3.4/scut-v0.3.4-linux-amd64.tar.gz": archive,
		"https://github.com/ajbeck/scut/releases/download/v0.3.4/checksums.txt":                  []byte("bad  scut-v0.3.4-linux-amd64.tar.gz\n"),
	})

	cmd := &Cmd{Version: "v0.3.4"}
	err := cmd.Run(io.Discard, afero.NewOsFs(), slog.Default())
	if err == nil {
		t.Fatal("Run() expected checksum error, got nil")
	}
	if !strings.Contains(err.Error(), "checksum verification failed") {
		t.Fatalf("Run() error = %v, want checksum verification failure", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old" {
		t.Fatalf("executable changed after checksum failure: %q", data)
	}
}

func resetUpdateTestState(t *testing.T) {
	t.Helper()
	oldExecutable := executable
	oldEvalSymlinks := evalSymlinks
	oldUserHomeDir := userHomeDir
	oldLookPath := lookPath
	oldHTTPClient := httpClient
	oldGoos, oldGoarch := goos, goarch
	oldVersion, oldMetadata := versionmeta.Version, versionmeta.BuildMetadata
	t.Cleanup(func() {
		executable = oldExecutable
		evalSymlinks = oldEvalSymlinks
		userHomeDir = oldUserHomeDir
		lookPath = oldLookPath
		httpClient = oldHTTPClient
		goos, goarch = oldGoos, oldGoarch
		versionmeta.Version = oldVersion
		versionmeta.BuildMetadata = oldMetadata
	})
	lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	versionmeta.BuildMetadata = ""
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func fakeHTTPClient(responses map[string][]byte) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, ok := responses[req.URL.String()]
		if !ok {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Status:     "404 Not Found",
				Body:       io.NopCloser(strings.NewReader("not found")),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}
}

func releaseArchive(t *testing.T, binary []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "scut", Mode: 0755, Size: int64(len(binary))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("output missing %q\n%s", needle, haystack)
	}
}
