// Package claude implements the "claude" agent command group.
package claude

import "github.com/ajbeck/botctrl/internal/cmd/claude/hook"

// Cmd is the Kong command group for "botctrl claude".
type Cmd struct {
	Hook       hook.Cmd      `cmd:"hook" help:"Hook event handlers. Called by Claude Code as subprocesses during lifecycle events."`
	StatusLine statusLineCmd `cmd:"status-line" help:"Render the Claude Code status bar. Reads session JSON from stdin, prints styled output to stdout."`
}
