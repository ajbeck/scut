//go:build goexperiment.jsonv2

// Package config implements the "botctrl claude config" command group.
package config

// Cmd is the Kong command group for "botctrl claude config".
type Cmd struct {
	Install   installCmd   `cmd:"install" help:"Write/merge botctrl hook and status-line entries into settings.json."`
	Uninstall uninstallCmd `cmd:"uninstall" help:"Remove botctrl entries from settings.json."`
	Status    statusCmd    `cmd:"status" help:"Show currently-installed botctrl entries in settings.json."`
}
