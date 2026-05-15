//go:build goexperiment.jsonv2

// Package config implements the "scut claude config" command group.
package config

// Cmd is the Kong command group for "scut claude config".
type Cmd struct {
	Install   installCmd   `cmd:"install" help:"Write/merge scut hook and status-line entries into settings.json."`
	Uninstall uninstallCmd `cmd:"uninstall" help:"Remove scut entries from settings.json."`
	Status    statusCmd    `cmd:"status" help:"Show currently-installed scut entries in settings.json."`
}
