package gotools

import (
	"bytes"
	"strings"
	"testing"
)

func TestDocCmdRequiresPackage(t *testing.T) {
	cmd := &docCmd{}
	var stdout bytes.Buffer

	err := cmd.Run(&stdout)
	if err == nil {
		t.Fatal("Run() error = nil, want package-required error")
	}
	if !strings.Contains(err.Error(), "package is required") {
		t.Fatalf("Run() error = %q, want package-required error", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}

func TestDocCmdWritesPlaceholder(t *testing.T) {
	cmd := &docCmd{Package: "encoding/json", Symbol: "Marshal"}
	var stdout bytes.Buffer

	if err := cmd.Run(&stdout); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := stdout.String(), "gotools doc placeholder: encoding/json Marshal\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
