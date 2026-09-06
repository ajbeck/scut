// The walle command is scut's repository-local task runner.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kong"

	versionmeta "github.com/ajbeck/scut/internal/version"
)

const (
	binaryName = "scut"
	buildDir   = "bin"
	mainPkg    = "./cmd/scut"
	versionPkg = "github.com/ajbeck/scut/internal/version"
)

var platforms = []struct {
	goos   string
	goarch string
}{
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
}

type cli struct {
	Build       buildCmd       `cmd:"" name:"build" help:"Compile the scut binary into bin/."`
	BuildAll    buildAllCmd    `cmd:"" name:"build-all" help:"Cross-compile scut for all supported release platforms."`
	BuildTarget buildTargetCmd `cmd:"" name:"build-platform" help:"Cross-compile scut for one OS and architecture."`
	Version     versionCmd     `cmd:"" name:"version" help:"Print the base version used for builds."`
	Test        testCmd        `cmd:"" name:"test" help:"Run all tests with the race detector enabled."`
	Vet         vetCmd         `cmd:"" name:"vet" help:"Run go vet across all packages."`
	Fmt         fmtCmd         `cmd:"" name:"fmt" help:"Run gofmt on all Go source files."`
	Docs        docsCmd        `cmd:"" name:"docs" help:"Build the Hugo documentation site into public/."`
	DocsCLIHelp docsCLIHelpCmd `cmd:"" name:"docs-cli-help" help:"Regenerate Hugo assets from scut help output."`
	LocalDeploy localDeployCmd `cmd:"" name:"local-deploy" help:"Build scut and copy it to a destination directory."`
}

type buildCmd struct{}

func (buildCmd) Run(ctx context.Context) error {
	return build(ctx, nil)
}

type buildTargetCmd struct {
	GOOS   string `arg:"" name:"goos" help:"Target operating system."`
	GOARCH string `arg:"" name:"goarch" help:"Target architecture."`
}

func (c buildTargetCmd) Run(ctx context.Context) error {
	return build(ctx, map[string]string{
		"GOOS":   c.GOOS,
		"GOARCH": c.GOARCH,
	})
}

type buildAllCmd struct{}

func (buildAllCmd) Run(ctx context.Context) error {
	for _, p := range platforms {
		if err := build(ctx, map[string]string{
			"GOOS":   p.goos,
			"GOARCH": p.goarch,
		}); err != nil {
			return err
		}
	}
	return nil
}

type versionCmd struct{}

func (versionCmd) Run(stdout io.Writer) {
	fmt.Fprintln(stdout, version())
}

type testCmd struct{}

func (testCmd) Run(ctx context.Context) error {
	return run(ctx, nil, "go", "test", "-race", "./...")
}

type vetCmd struct{}

func (vetCmd) Run(ctx context.Context) error {
	return run(ctx, nil, "go", "vet", "./...")
}

type fmtCmd struct{}

func (fmtCmd) Run(ctx context.Context) error {
	return run(ctx, nil, "gofmt", "-w", ".")
}

type docsCmd struct{}

func (docsCmd) Run(ctx context.Context) error {
	if err := docsCLIHelp(ctx); err != nil {
		return err
	}
	return run(ctx, nil, "hugo", "--source", "docs", "--gc", "--minify")
}

type docsCLIHelpCmd struct{}

func (docsCLIHelpCmd) Run(ctx context.Context) error {
	return docsCLIHelp(ctx)
}

type localDeployCmd struct {
	Destination string `arg:"" type:"path" help:"Directory to copy the built scut binary into."`
}

func (c localDeployCmd) Run(ctx context.Context, stdout io.Writer) error {
	if err := build(ctx, nil); err != nil {
		return err
	}
	src := filepath.Join(buildDir, binaryName)
	dst := filepath.Join(c.Destination, binaryName)
	if err := run(ctx, nil, "cp", src, dst); err != nil {
		return err
	}
	abs, _ := filepath.Abs(dst)
	fmt.Fprintf(stdout, "deployed %s -> %s\n", binaryName, abs)
	return nil
}

type cliHelpDoc struct {
	name    string
	command string
	args    []string
}

var cliHelpDocs = []cliHelpDoc{
	{name: "scut", command: "scut --help", args: []string{"--help"}},
	{name: "scut-version", command: "scut version --help", args: []string{"version", "--help"}},
	{name: "scut-claude", command: "scut claude --help", args: []string{"claude", "--help"}},
	{name: "scut-claude-status-line", command: "scut claude status-line --help", args: []string{"claude", "status-line", "--help"}},
	{name: "scut-claude-config", command: "scut claude config --help", args: []string{"claude", "config", "--help"}},
	{name: "scut-claude-config-install", command: "scut claude config install --help", args: []string{"claude", "config", "install", "--help"}},
	{name: "scut-claude-config-uninstall", command: "scut claude config uninstall --help", args: []string{"claude", "config", "uninstall", "--help"}},
	{name: "scut-claude-config-status", command: "scut claude config status --help", args: []string{"claude", "config", "status", "--help"}},
	{name: "scut-claude-hook", command: "scut claude hook --help", args: []string{"claude", "hook", "--help"}},
	{name: "scut-codex", command: "scut codex --help", args: []string{"codex", "--help"}},
	{name: "scut-codex-config", command: "scut codex config --help", args: []string{"codex", "config", "--help"}},
	{name: "scut-codex-config-install", command: "scut codex config install --help", args: []string{"codex", "config", "install", "--help"}},
	{name: "scut-codex-config-uninstall", command: "scut codex config uninstall --help", args: []string{"codex", "config", "uninstall", "--help"}},
	{name: "scut-codex-config-status", command: "scut codex config status --help", args: []string{"codex", "config", "status", "--help"}},
	{name: "scut-codex-hook", command: "scut codex hook --help", args: []string{"codex", "hook", "--help"}},
	{name: "scut-init", command: "scut init --help", args: []string{"init", "--help"}},
	{name: "scut-doctor", command: "scut doctor --help", args: []string{"doctor", "--help"}},
	{name: "scut-update", command: "scut update --help", args: []string{"update", "--help"}},
	{name: "scut-format", command: "scut format --help", args: []string{"format", "--help"}},
	{name: "scut-format-go", command: "scut format go --help", args: []string{"format", "go", "--help"}},
	{name: "scut-format-markdown", command: "scut format markdown --help", args: []string{"format", "markdown", "--help"}},
	{name: "scut-gotools", command: "scut gotools --help", args: []string{"gotools", "--help"}},
	{name: "scut-gotools-doc", command: "scut gotools doc --help", args: []string{"gotools", "doc", "--help"}},
	{name: "scut-gotools-cache", command: "scut gotools cache --help", args: []string{"gotools", "cache", "--help"}},
	{name: "scut-gotools-cache-path", command: "scut gotools cache path --help", args: []string{"gotools", "cache", "path", "--help"}},
	{name: "scut-gotools-cache-list", command: "scut gotools cache list --help", args: []string{"gotools", "cache", "list", "--help"}},
	{name: "scut-gotools-cache-verify", command: "scut gotools cache verify --help", args: []string{"gotools", "cache", "verify", "--help"}},
	{name: "scut-gotools-cache-remove", command: "scut gotools cache remove --help", args: []string{"gotools", "cache", "remove", "--help"}},
	{name: "scut-gotools-cache-clean", command: "scut gotools cache clean --help", args: []string{"gotools", "cache", "clean", "--help"}},
	{name: "scut-gotools-cache-prune", command: "scut gotools cache prune --help", args: []string{"gotools", "cache", "prune", "--help"}},
	{name: "scut-logging", command: "scut logging --help", args: []string{"logging", "--help"}},
	{name: "scut-logging-clean", command: "scut logging clean --help", args: []string{"logging", "clean", "--help"}},
	{name: "scut-mcp", command: "scut mcp --help", args: []string{"mcp", "--help"}},
	{name: "scut-mcp-aws-proxy", command: "scut mcp aws-proxy --help", args: []string{"mcp", "aws-proxy", "--help"}},
}

func main() {
	var c cli
	parser := kong.Must(&c,
		kong.Name("walle"),
		kong.Description("Repository-local task runner for scut development."),
		kong.BindTo(context.Background(), (*context.Context)(nil)),
		kong.BindTo(os.Stdout, (*io.Writer)(nil)),
		kong.ConfigureHelp(kong.HelpOptions{
			NoExpandSubcommands: true,
			FlagsLast:           true,
			Compact:             true,
		}),
	)
	ctx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)
	ctx.FatalIfErrorf(ctx.Run())
}

func build(ctx context.Context, env map[string]string) error {
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return err
	}
	goos, goarch := "", ""
	if env != nil {
		goos = env["GOOS"]
		goarch = env["GOARCH"]
	}
	out := filepath.Join(buildDir, binaryPath(goos, goarch))
	return run(ctx, env, "go", "build", "-ldflags", ldflags(), "-o", out, mainPkg)
}

func docsCLIHelp(ctx context.Context) error {
	if err := build(ctx, nil); err != nil {
		return err
	}

	dir := filepath.Join("docs", "assets", "cli-help")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	binary := filepath.Join(".", buildDir, binaryName)
	for _, doc := range cliHelpDocs {
		out, err := output(ctx, nil, binary, doc.args...)
		if err != nil {
			return fmt.Errorf("generate CLI help for %s: %w", doc.command, err)
		}
		out = strings.TrimRight(out, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(dir, doc.name+".txt"), []byte(out), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func version() string {
	if v := strings.TrimSpace(os.Getenv("RELEASE_VERSION")); v != "" {
		return v
	}
	return versionmeta.Version
}

func buildMetadata() string {
	if m := os.Getenv("BUILD_METADATA"); m != "" {
		return m
	}
	return "local:" + time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func ldflags() string {
	flags := "-X " + versionPkg + ".Version=" + version() + " -X " + versionPkg + ".BuildMetadata=" + buildMetadata()
	if strings.EqualFold(os.Getenv("RELEASE"), "true") {
		flags = "-s -w " + flags
	}
	return flags
}

func binaryPath(goos, goarch string) string {
	if goos == "" || goarch == "" {
		return binaryName
	}
	return fmt.Sprintf("%s-%s-%s", binaryName, goos, goarch)
}

func run(ctx context.Context, extraEnv map[string]string, name string, args ...string) error {
	cmd := command(ctx, extraEnv, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func output(ctx context.Context, extraEnv map[string]string, name string, args ...string) (string, error) {
	cmd := command(ctx, extraEnv, name, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return stdout.String(), nil
}

func command(ctx context.Context, extraEnv map[string]string, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = mergedEnv(extraEnv)
	return cmd
}

func mergedEnv(extra map[string]string) []string {
	env := os.Environ()
	env = append(env, "GOEXPERIMENT=jsonv2")
	for key, value := range extra {
		env = append(env, key+"="+value)
	}
	return env
}
