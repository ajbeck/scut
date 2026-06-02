//go:build goexperiment.jsonv2

// Package update implements the "scut update" command.
package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	json "encoding/json/v2"

	"github.com/spf13/afero"
	"golang.org/x/mod/semver"

	versionmeta "github.com/ajbeck/scut/internal/version"
)

const repo = "ajbeck/scut"

// Cmd is the Kong command for "scut update".
type Cmd struct {
	Version       string `arg:"" optional:"" help:"Target release version. Defaults to latest stable release." placeholder:"VERSION"`
	TargetVersion string `help:"Target release version. Defaults to latest stable release." name:"target-version" placeholder:"VERSION"`
	DryRun        bool   `help:"Show detected install method and planned action without changing files." name:"dry-run"`
}

type installMethod string

const (
	methodScript   installMethod = "install-script"
	methodHomebrew installMethod = "homebrew"
	methodSource   installMethod = "source"
	methodUnknown  installMethod = "unknown"
)

type plan struct {
	CurrentVersion string
	TargetVersion  string
	ExecutablePath string
	ResolvedPath   string
	Method         installMethod
	Action         string
}

type latestReleaseResponse struct {
	TagName string `json:"tag_name"`
}

var (
	executable   = os.Executable
	evalSymlinks = filepath.EvalSymlinks
	userHomeDir  = os.UserHomeDir
	lookPath     = exec.LookPath
	httpClient   = http.DefaultClient
	goos         = runtime.GOOS
	goarch       = runtime.GOARCH
)

// Run updates script-installed scut binaries and prints guidance for managed installs.
func (c *Cmd) Run(stdout io.Writer, fs afero.Fs, logger *slog.Logger) error {
	_ = logger
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p, err := c.plan(ctx, fs)
	if err != nil {
		return err
	}
	writePlan(stdout, p)
	if c.DryRun {
		return nil
	}

	switch p.Method {
	case methodScript:
		if sameVersion(p.CurrentVersion, p.TargetVersion) {
			fmt.Fprintf(stdout, "scut is already at %s\n", p.TargetVersion)
			return nil
		}
		if err := installRelease(ctx, fs, p.TargetVersion, p.ExecutablePath); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "updated scut from %s to %s at %s\n", p.CurrentVersion, p.TargetVersion, p.ExecutablePath)
		return nil
	case methodHomebrew, methodSource, methodUnknown:
		return nil
	default:
		return fmt.Errorf("unknown install method %q", p.Method)
	}
}

func (c *Cmd) plan(ctx context.Context, fs afero.Fs) (plan, error) {
	exe, err := executable()
	if err != nil {
		return plan{}, fmt.Errorf("resolve executable path: %w", err)
	}
	resolved := exe
	if real, err := evalSymlinks(exe); err == nil {
		resolved = real
	}

	requested, err := c.requestedVersion()
	if err != nil {
		return plan{}, err
	}
	target, err := resolveTargetVersion(ctx, requested)
	if err != nil {
		return plan{}, err
	}
	current := versionmeta.String()
	method := detectInstallMethod(fs, exe, resolved, current)
	p := plan{
		CurrentVersion: current,
		TargetVersion:  target,
		ExecutablePath: exe,
		ResolvedPath:   resolved,
		Method:         method,
	}
	p.Action = plannedAction(p)
	return p, nil
}

func (c *Cmd) requestedVersion() (string, error) {
	if c.Version != "" && c.TargetVersion != "" {
		return "", fmt.Errorf("pass either VERSION or --target-version, not both")
	}
	if c.TargetVersion != "" {
		return c.TargetVersion, nil
	}
	return c.Version, nil
}

func resolveTargetVersion(ctx context.Context, requested string) (string, error) {
	if requested == "" || requested == "latest" {
		return latestRelease(ctx)
	}
	if !strings.HasPrefix(requested, "v") {
		requested = "v" + requested
	}
	if !semver.IsValid(requested) {
		return "", fmt.Errorf("invalid target version %q", requested)
	}
	return semver.Canonical(requested), nil
}

func latestRelease(ctx context.Context) (string, error) {
	url := "https://api.github.com/repos/" + repo + "/releases/latest"
	body, err := httpGet(ctx, url)
	if err != nil {
		return "", fmt.Errorf("resolve latest release: %w", err)
	}
	var response latestReleaseResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("parse latest release response: %w", err)
	}
	if response.TagName == "" {
		return "", fmt.Errorf("latest release response missing tag_name")
	}
	if !semver.IsValid(response.TagName) {
		return "", fmt.Errorf("latest release tag %q is not valid semver", response.TagName)
	}
	return semver.Canonical(response.TagName), nil
}

func detectInstallMethod(fs afero.Fs, exe, resolved, current string) installMethod {
	exe = filepath.Clean(exe)
	resolved = filepath.Clean(resolved)
	lowerExe := strings.ToLower(exe)
	lowerResolved := strings.ToLower(resolved)

	if isHomebrewPath(lowerExe) || isHomebrewPath(lowerResolved) {
		return methodHomebrew
	}
	if isDevVersion(current) || isRepoBuild(fs, exe) || isRepoBuild(fs, resolved) || isGoInstallPath(exe) || isGoInstallPath(resolved) {
		return methodSource
	}
	if isScriptPath(exe) || isScriptPath(resolved) {
		return methodScript
	}
	return methodUnknown
}

func isHomebrewPath(path string) bool {
	return strings.Contains(path, "/cellar/scut/") ||
		strings.Contains(path, "/homebrew/cellar/scut/") ||
		strings.Contains(path, "/opt/homebrew/") ||
		strings.Contains(path, "/usr/local/homebrew/")
}

func isDevVersion(v string) bool {
	base := strings.Split(v, "+")[0]
	return base == "v0.0.0-dev" || strings.Contains(base, "-dev")
}

func isRepoBuild(fs afero.Fs, path string) bool {
	if filepath.Base(path) != "scut" || filepath.Base(filepath.Dir(path)) != "bin" {
		return false
	}
	root := filepath.Dir(filepath.Dir(path))
	data, err := afero.ReadFile(fs, filepath.Join(root, "go.mod"))
	return err == nil && bytes.Contains(data, []byte("module github.com/ajbeck/scut"))
}

func isGoInstallPath(path string) bool {
	home, err := userHomeDir()
	if err != nil {
		return false
	}
	return filepath.Clean(path) == filepath.Join(home, "go", "bin", "scut")
}

func isScriptPath(path string) bool {
	home, err := userHomeDir()
	if err == nil && filepath.Clean(path) == filepath.Join(home, ".local", "bin", "scut") {
		return true
	}
	return filepath.Base(path) == "scut" &&
		(strings.HasSuffix(filepath.ToSlash(filepath.Dir(path)), "/.local/bin") ||
			filepath.Clean(path) == "/usr/local/bin/scut")
}

func plannedAction(p plan) string {
	switch p.Method {
	case methodScript:
		asset, err := assetName(p.TargetVersion)
		if err != nil {
			return "unsupported platform for release update"
		}
		return fmt.Sprintf("download %s, verify checksums.txt, and replace %s", asset, p.ExecutablePath)
	case methodHomebrew:
		return homebrewAction()
	case methodSource:
		if isDevVersion(p.CurrentVersion) {
			return "source/development build detected; rebuild with mage build or install with go install github.com/ajbeck/scut@latest"
		}
		return "source-managed install detected; update with go install github.com/ajbeck/scut@latest"
	default:
		return "install method is unknown; reinstall with curl -fsSL https://install-scut.ajbeck.dev | sh or use your package manager"
	}
}

func homebrewAction() string {
	formula := "scut"
	if _, err := lookPath("brew"); err == nil {
		formula = "ajbeck/tap/scut"
	}
	return "Homebrew-managed install detected; update with:\n  brew update\n  brew upgrade " + formula
}

func writePlan(stdout io.Writer, p plan) {
	fmt.Fprintf(stdout, "current version: %s\n", p.CurrentVersion)
	fmt.Fprintf(stdout, "target version: %s\n", p.TargetVersion)
	fmt.Fprintf(stdout, "binary path: %s\n", p.ExecutablePath)
	if p.ResolvedPath != p.ExecutablePath {
		fmt.Fprintf(stdout, "resolved path: %s\n", p.ResolvedPath)
	}
	fmt.Fprintf(stdout, "install method: %s\n", p.Method)
	fmt.Fprintf(stdout, "action: %s\n", p.Action)
}

func installRelease(ctx context.Context, fs afero.Fs, version, executablePath string) error {
	asset, err := assetName(version)
	if err != nil {
		return err
	}
	baseURL := "https://github.com/" + repo + "/releases/download/" + version
	archive, err := httpGet(ctx, baseURL+"/"+asset)
	if err != nil {
		return fmt.Errorf("download %s: %w", asset, err)
	}
	checksums, err := httpGet(ctx, baseURL+"/checksums.txt")
	if err != nil {
		return fmt.Errorf("download checksums.txt: %w", err)
	}
	if err := verifyChecksum(asset, archive, checksums); err != nil {
		return err
	}
	binary, err := extractBinary(archive)
	if err != nil {
		return err
	}
	return replaceExecutable(fs, executablePath, binary)
}

func assetName(version string) (string, error) {
	osName, err := releaseOS(goos)
	if err != nil {
		return "", err
	}
	archName, err := releaseArch(goarch)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("scut-%s-%s-%s.tar.gz", version, osName, archName), nil
}

func releaseOS(value string) (string, error) {
	switch value {
	case "darwin", "linux":
		return value, nil
	default:
		return "", fmt.Errorf("unsupported operating system: %s", value)
	}
}

func releaseArch(value string) (string, error) {
	switch value {
	case "amd64", "arm64":
		return value, nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", value)
	}
}

func httpGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s returned %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func verifyChecksum(asset string, archive, checksums []byte) error {
	expected := checksumForAsset(asset, checksums)
	if expected == "" {
		return fmt.Errorf("checksum for %s not found", asset)
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(archive))
	if actual != expected {
		return fmt.Errorf("checksum verification failed for %s", asset)
	}
	return nil
}

func checksumForAsset(asset string, checksums []byte) string {
	for line := range strings.Lines(string(checksums)) {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			return fields[0]
		}
	}
	return ""
}

func extractBinary(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("read release archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read release archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(header.Name) != "scut" {
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read scut from release archive: %w", err)
		}
		return body, nil
	}
	return nil, fmt.Errorf("release archive missing scut binary")
}

func replaceExecutable(fs afero.Fs, path string, binary []byte) error {
	dir := filepath.Dir(path)
	tmp, err := afero.TempFile(fs, dir, ".scut-update-*")
	if err != nil {
		return fmt.Errorf("create temporary executable: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = fs.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(binary); err != nil {
		tmp.Close()
		return fmt.Errorf("write temporary executable: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary executable: %w", err)
	}
	if err := fs.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("chmod temporary executable: %w", err)
	}
	if err := fs.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace executable: %w", err)
	}
	cleanup = false
	return nil
}

func sameVersion(current, target string) bool {
	base := strings.Split(current, "+")[0]
	if !semver.IsValid(base) || !semver.IsValid(target) {
		return current == target
	}
	return semver.Canonical(base) == semver.Canonical(target)
}
