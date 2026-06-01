---

title: "Kong Base Setup"
description: "How commands are wired, parsed, and executed through Kong."
kicker: "Contributing"
tags: ["Kong", "CLI"]
weight: 20
---

Kong maps Go structs to the command tree. Each command is a small struct with a `Run` method. Dependencies such as `io.Reader`, `io.Writer`, filesystem handles, and loggers are passed through Kong bindings.

## Command shape

```go
type someCmd struct {
    Flag bool `help:"Enable the behavior."`
}

func (c *someCmd) Run(stdout io.Writer) error {
    _, err := fmt.Fprintln(stdout, "ok")
    return err
}
```

Top-level command groups compose these structs:

```go
type CLI struct {
    Init   initcmd.Cmd `cmd:"" help:"Set up agent hooks."`
    Doctor doctor.Cmd  `cmd:"" help:"Run diagnostics."`
    Claude claude.Cmd  `cmd:"" help:"Claude Code integration."`
    Codex  codex.Cmd   `cmd:"" help:"Codex integration."`
}
```

## Parse errors

`main()` performs parsing explicitly instead of letting Kong exit immediately. That gives scut one place to log parse errors before rendering Kong's normal CLI error.

The production entrypoint follows this pattern:

```go
var c cli
parser := kong.Must(&c,
    kong.Name("scut"),
    kong.Description("CLI tool for managing AI coding agents..."),
    kong.Vars{"version": version.String()},
    kong.BindTo(os.Stdin, (*io.Reader)(nil)),
    kong.BindTo(os.Stdout, (*io.Writer)(nil)),
    kong.BindTo(afero.NewOsFs(), (*afero.Fs)(nil)),
)

ctx, err := parser.Parse(os.Args[1:])
if err != nil {
    logging.LogParseError(os.Args, err)
    parser.FatalIfErrorf(err)
}

logger, logCloser := c.openLogger(ctx.Command())
if logCloser != nil {
    defer logCloser.Close()
}
ctx.FatalIfErrorf(ctx.Run(logger))
```

Keep parse and run separate. Parse failures happen before a command is selected, so this is the only place to record malformed hook invocations in scut's parse-error log.

## Dependency injection

Kong resolves `Run` method parameters by type. scut uses static parser bindings for process and filesystem dependencies, then passes the logger as a run-time binding after parsing logging flags.

| Binding                                          | Interface      | Purpose                                                                     |
| ------------------------------------------------ | -------------- | --------------------------------------------------------------------------- |
| `kong.BindTo(os.Stdin, (*io.Reader)(nil))`       | `io.Reader`    | Hook commands read JSON payloads from stdin.                                |
| `kong.BindTo(os.Stdout, (*io.Writer)(nil))`      | `io.Writer`    | Hook/config commands write JSON or terminal output.                         |
| `kong.BindTo(afero.NewOsFs(), (*afero.Fs)(nil))` | `afero.Fs`     | Commands that read/write project files stay testable with `afero.MemMapFs`. |
| `ctx.Run(logger)`                                | `*slog.Logger` | Logger is selected after parsing `--log` and `--log-level`.                 |

Prefer injecting a dependency into `Run` over reaching for package globals. That keeps command behavior easy to unit test and avoids special cases for hook subprocesses.

## Adding commands

When adding a command:

1. Put implementation under `internal/cmd/<area>` unless a public package is explicitly needed.
2. Add the command struct to the owning command group.
3. Keep `Run` dependencies explicit and injectable.
4. Add focused tests around parsing and behavior.
5. Update Usage or Contributing docs when the command surface or behavior changes.

For a new leaf command, define an unexported struct with a `Run` method and add it to the parent `Cmd` struct with an explicit command name:

```go
type myNewCmd struct {
    DryRun bool `help:"Print planned changes without writing." name:"dry-run"`
}

func (c *myNewCmd) Run(stdout io.Writer, fs afero.Fs, logger *slog.Logger) error {
    // implementation
    return nil
}

type Cmd struct {
    MyNew myNewCmd `cmd:"my-new" help:"Does the new thing."`
}
```

For a new command group, create `internal/cmd/<group>/` with an exported `Cmd` struct, then embed that `Cmd` in its parent. Group structs normally do not have `Run` methods; leaf structs do.

## Struct tags

Tags used most often in this repo:

| Tag              | Purpose                                      | Example                             |
| ---------------- | -------------------------------------------- | ----------------------------------- |
| `cmd:""`         | Marks a struct field as a command.           | `cmd:"post-tool-use"`               |
| `help:""`        | Short help text shown in generated `--help`. | `help:"Handle PostToolUse events."` |
| `name:""`        | Overrides a flag name.                       | `name:"dry-run"`                    |
| `short:""`       | Adds a single-letter flag alias.             | `short:"v"`                         |
| `arg:""`         | Marks a positional argument.                 | `arg:""`                            |
| `optional:""`    | Makes a positional argument optional.        | `optional:""`                       |
| `default:""`     | Sets a default value.                        | `default:"project"`                 |
| `enum:""`        | Restricts a flag to listed values.           | `enum:"project,user"`               |
| `placeholder:""` | Controls placeholder text in help.           | `placeholder:"LEVEL"`               |
| `hidden:""`      | Hides a field from help output.              | `hidden:""`                         |

Every user-facing command/flag tag feeds the generated CLI reference. After command-tree changes, run `mage docs` so generated help assets are refreshed.
