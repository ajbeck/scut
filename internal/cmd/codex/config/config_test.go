//go:build goexperiment.jsonv2

package config_test

import (
	"testing"

	"github.com/alecthomas/kong"

	"github.com/ajbeck/scut/internal/cmd/codex/config"
)

type kongSmokeCli struct {
	Config config.Cmd `cmd:""`
}

func TestKongWiring(t *testing.T) {
	var cli kongSmokeCli
	parser := kong.Must(&cli, kong.Name("scut"))

	for _, args := range [][]string{
		{"config", "install", "--dry-run", "--only=post-tool-use", "--scope=project"},
		{"config", "uninstall", "--scope=user"},
		{"config", "status", "--scope=both", "--json"},
	} {
		if _, err := parser.Parse(args); err != nil {
			t.Fatalf("Parse(%v) error: %v", args, err)
		}
	}
}
