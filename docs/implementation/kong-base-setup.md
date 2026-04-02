# Implementation

## CLI Framework

botctrl uses [kong](https://github.com/alecthomas/kong) for CLI parsing. Kong models the entire command tree as nested Go structs — there is no imperative flag registration. The struct _is_ the schema.

### Entrypoint

`cmd/botctrl/main.go` defines the root `cli` struct and calls `kong.Parse`:

```go
type cli struct {
    Version versionFlag `name:"version" help:"Print version and exit." short:"v"`
    Claude  claude.Cmd  `cmd:"claude" help:"Claude Code agent commands — hooks, status line, and configuration."`
}

func main() {
    var c cli
    ctx := kong.Parse(&c,
        kong.Name("botctrl"),
        kong.Description("CLI tool for managing AI coding assistants."),
        kong.Vars{"version": version.String()},
        kong.BindTo(os.Stdin, (*io.Reader)(nil)),
        kong.BindTo(os.Stdout, (*io.Writer)(nil)),
        kong.BindTo(afero.NewOsFs(), (*afero.Fs)(nil)),
    )
    ctx.FatalIfErrorf(ctx.Run())
}
```

`kong.Parse` parses `os.Args`, resolves the selected command, and returns a `*kong.Context`. `ctx.Run()` finds the selected command's `Run()` method and calls it, injecting any bound dependencies as arguments.

### Dependency Injection via Bind

We use `kong.BindTo` to wire dependencies that commands receive through their `Run()` method signature. Kong matches `Run()` parameters by type — if a parameter is `io.Reader`, Kong looks up what was bound to that interface and injects it.

Current bindings registered at parse time:

| Binding | Interface | Concrete Value | Purpose |
|---------|-----------|----------------|---------|
| `kong.BindTo(os.Stdin, (*io.Reader)(nil))` | `io.Reader` | `os.Stdin` | Commands read input (e.g., hook JSON payloads) from stdin |
| `kong.BindTo(os.Stdout, (*io.Writer)(nil))` | `io.Writer` | `os.Stdout` | Commands write output (e.g., hook JSON responses) to stdout |
| `kong.BindTo(afero.NewOsFs(), (*afero.Fs)(nil))` | `afero.Fs` | `afero.OsFs` | Commands that need filesystem access (e.g., post-tool-use formatting) |

The `(*io.Reader)(nil)` syntax is a nil pointer used only for its type — Kong reflects on it to determine the interface. The concrete value (`os.Stdin`) is what gets injected at runtime.

This makes commands testable without touching real stdio:

```go
func TestPreToolUse(t *testing.T) {
    input := strings.NewReader(`{"session_id":"test",...}`)
    var output bytes.Buffer
    cmd := &preToolUseCmd{}
    if err := cmd.Run(input, &output); err != nil {
        t.Fatal(err)
    }
    // assert on output.Bytes()
}
```

#### Adding New Bindings

To add a new dependency available to all commands:

1. Register it in `main()` with `kong.BindTo(impl, (*InterfaceType)(nil))` for interfaces, or `kong.Bind(value)` for concrete types.
2. Add the type as a parameter to any command's `Run()` method. Kong resolves it automatically.

```go
// In main():
kong.Bind(logger)

// In a command:
func (c *myCmd) Run(stdin io.Reader, stdout io.Writer, log *slog.Logger) error {
```

### Version Flag

The `--version` / `-v` flag uses Kong's `BeforeReset` lifecycle hook. When the flag is set, `BeforeReset` fires before any command resolution — it prints the version string and exits. This avoids requiring a dedicated `version` subcommand.

### Kong Vars

`kong.Vars{"version": version.String()}` registers key-value pairs available to struct tags via `${version}` interpolation and to lifecycle hooks via the `kong.Vars` map. Currently used only for version output.

## Command Structure

### Layout

Commands live under `internal/cmd/` organized by agent, then by capability:

```
cmd/botctrl/main.go                    # Entrypoint, root cli struct, bindings
internal/cmd/claude/claude.go          # "botctrl claude" agent group
internal/cmd/claude/hook/hook.go       # "botctrl claude hook" subcommands
```

Each level exports a `Cmd` struct that the parent embeds with a `cmd:""` tag.

### How Commands Map to Structs

Kong's command tree is a direct reflection of struct nesting:

```
botctrl claude hook pre-tool-use
   │      │     │        │
   cli    │     │        └─ preToolUseCmd (leaf — has Run())
          │     └─ hook.Cmd struct field
          └─ claude.Cmd struct field
```

- **Group nodes** (claude, hook) are exported `Cmd` structs with `cmd:""` tagged fields for their children. They have no `Run()` method.
- **Leaf commands** (pre-tool-use, session-start, etc.) are unexported structs with a `Run(...) error` method. They are the actual execution targets.

### Adding a New Command

#### Adding a leaf command to an existing group

1. Define an unexported struct in the group's package:

```go
type myNewCmd struct{}

func (c *myNewCmd) Run(stdin io.Reader, stdout io.Writer) error {
    // implementation
    return nil
}
```

2. Add it as a field on the group's `Cmd` struct:

```go
type Cmd struct {
    // ... existing commands
    MyNew myNewCmd `cmd:"my-new" help:"Does the new thing."`
}
```

The `cmd:"my-new"` tag sets the CLI name. Kong lowercases and hyphenates automatically if you omit the tag value, but explicit names are clearer.

#### Adding a new command group

1. Create a new package under `internal/cmd/`:

```
internal/cmd/mygroup/mygroup.go
```

2. Define an exported `Cmd` struct with child commands:

```go
package mygroup

type Cmd struct {
    SubCmd subCmd `cmd:"sub" help:"A subcommand."`
}

type subCmd struct{}

func (c *subCmd) Run(stdin io.Reader, stdout io.Writer) error {
    return nil
}
```

3. Add the group to its parent's `Cmd` struct:

```go
// In cmd/botctrl/main.go or the parent group package:
type cli struct {
    // ... existing
    MyGroup mygroup.Cmd `cmd:"my-group" help:"My new group."`
}
```

### Struct Tag Reference

Tags used on command/flag struct fields:

| Tag | Purpose | Example |
|-----|---------|---------|
| `cmd:""` | Marks a struct field as a subcommand | `cmd:"pre-tool-use"` |
| `help:""` | Short help text shown in `--help` | `help:"Handle PreToolUse events."` |
| `name:""` | Overrides the flag/command name | `name:"version"` |
| `short:""` | Single-character flag alias | `short:"v"` |
| `arg:""` | Marks a field as a positional argument | `arg:""` |
| `required:""` | Makes a flag/arg mandatory | `required:""` |
| `default:""` | Sets a default value | `default:"json"` |
| `enum:""` | Restricts to a set of values | `enum:"json,text"` |
| `env:""` | Reads default from env var | `env:"BOTCTRL_FORMAT"` |
| `hidden:""` | Hides from help output | `hidden:""` |

## Build

See `magefiles/` for build targets. `mage build` compiles with ldflags that inject build metadata into `internal/version`. All mage targets set `GOEXPERIMENT=jsonv2` via `init()` in `magefiles/helpers.go`.
