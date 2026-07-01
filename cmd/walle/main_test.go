package main

import (
	"bytes"
	"io"
	"testing"

	"github.com/alecthomas/kong"
)

func TestCommandsParse(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		command string
	}{
		{
			name:    "build",
			args:    []string{"build"},
			command: "build",
		},
		{
			name:    "build platform",
			args:    []string{"build-platform", "darwin", "arm64"},
			command: "build-platform <goos> <goarch>",
		},
		{
			name:    "docs cli help",
			args:    []string{"docs-cli-help"},
			command: "docs-cli-help",
		},
		{
			name:    "local deploy",
			args:    []string{"local-deploy", "/tmp/bin"},
			command: "local-deploy <destination>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c cli
			var stdout bytes.Buffer
			parser := kong.Must(&c,
				kong.Name("walle"),
				kong.BindTo(&stdout, (*io.Writer)(nil)),
			)
			ctx, err := parser.Parse(tt.args)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if got := ctx.Command(); got != tt.command {
				t.Fatalf("Command() = %q, want %q", got, tt.command)
			}
		})
	}
}
