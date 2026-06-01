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

## Adding commands

When adding a command:

1. Put implementation under `internal/cmd/<area>` unless a public package is explicitly needed.
2. Add the command struct to the owning command group.
3. Keep `Run` dependencies explicit and injectable.
4. Add focused tests around parsing and behavior.
5. Update Usage or Contributing docs when the command surface or behavior changes.
