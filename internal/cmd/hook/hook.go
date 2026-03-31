// Package hook implements the "hook" subcommand group.
package hook

import "github.com/ajbeck/botctrl/internal/cmd/hook/claude"

// Cmd is the Kong command group for "botctrl hook".
type Cmd struct {
	Claude claude.Cmd `cmd:"claude" help:"Claude Code hook event handlers. Each subcommand handles one event type — reads the event payload from stdin, writes a JSON response to stdout."`
}
