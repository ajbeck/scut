//go:build goexperiment.jsonv2

// Package config implements the "scut codex config" command group.
package config

// Cmd is the Kong command group for "scut codex config".
type Cmd struct {
	Install   installCmd   `cmd:"install" help:"Write/merge scut hook entries into hooks.json."`
	Uninstall uninstallCmd `cmd:"uninstall" help:"Remove scut entries from hooks.json."`
	Status    statusCmd    `cmd:"status" help:"Show currently-installed scut entries in hooks.json."`
}
