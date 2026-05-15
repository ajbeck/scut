//go:build goexperiment.jsonv2

package config

import (
	"bytes"
	"testing"

	"github.com/spf13/afero"
)

func TestMarshalSettings_idempotent_bytes(t *testing.T) {
	s := Settings{
		StatusLine: &StatusLine{Type: "command", Command: "botctrl claude status-line"},
		Hooks: map[string][]HookGroup{
			"PostToolUse": {
				{
					Matcher: "Write|Edit",
					Hooks:   []HookEntry{{Type: "command", Command: "botctrl claude hook post-tool-use", StatusMessage: "Formatting..."}},
				},
			},
		},
	}

	first, err := marshalSettings(s)
	if err != nil {
		t.Fatalf("first marshalSettings: %v", err)
	}
	second, err := marshalSettings(s)
	if err != nil {
		t.Fatalf("second marshalSettings: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("marshalSettings not idempotent:\nfirst:  %q\nsecond: %q", first, second)
	}
}

func TestMarshalSettings_trailing_newline(t *testing.T) {
	s := Settings{}
	data, err := marshalSettings(s)
	if err != nil {
		t.Fatalf("marshalSettings: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Errorf("marshalSettings output does not end with newline: %q", data)
	}
}

func TestReadWriteSettings_foreign_key_round_trip(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/tmp/settings.json"

	// Write a settings file with foreign keys.
	original := `{
  "statusLine": {"type": "command", "command": "botctrl claude status-line"},
  "hooks": {},
  "allowedTools": ["bash", "read"],
  "zebra": true
}
`
	if err := afero.WriteFile(fs, path, []byte(original), 0644); err != nil {
		t.Fatalf("seeding file: %v", err)
	}

	s, err := readSettings(fs, path)
	if err != nil {
		t.Fatalf("readSettings: %v", err)
	}

	// Foreign keys must be present.
	if s.Foreign["allowedTools"] == nil {
		t.Errorf("foreign key %q missing after readSettings", "allowedTools")
	}
	if s.Foreign["zebra"] == nil {
		t.Errorf("foreign key %q missing after readSettings", "zebra")
	}

	// Write back and verify foreign keys are preserved.
	if err := writeSettings(fs, path, s); err != nil {
		t.Fatalf("writeSettings: %v", err)
	}

	s2, err := readSettings(fs, path)
	if err != nil {
		t.Fatalf("readSettings after write: %v", err)
	}
	if s2.Foreign["allowedTools"] == nil {
		t.Errorf("foreign key %q lost after write/read round-trip", "allowedTools")
	}
	if s2.Foreign["zebra"] == nil {
		t.Errorf("foreign key %q lost after write/read round-trip", "zebra")
	}
}

func TestReadWriteSettings_foreign_key_sorted(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/tmp/settings_sorted.json"

	// Write a settings file where foreign keys are intentionally out of alpha order.
	original := `{"zebra": 1, "apple": 2, "mango": 3}` + "\n"
	if err := afero.WriteFile(fs, path, []byte(original), 0644); err != nil {
		t.Fatalf("seeding file: %v", err)
	}

	s, err := readSettings(fs, path)
	if err != nil {
		t.Fatalf("readSettings: %v", err)
	}

	data, err := marshalSettings(s)
	if err != nil {
		t.Fatalf("marshalSettings: %v", err)
	}

	// After marshalling, "apple" must appear before "mango" before "zebra".
	applePos := bytes.Index(data, []byte(`"apple"`))
	mangoPos := bytes.Index(data, []byte(`"mango"`))
	zebraPos := bytes.Index(data, []byte(`"zebra"`))

	if applePos < 0 || mangoPos < 0 || zebraPos < 0 {
		t.Fatalf("expected foreign keys in output, got: %q", data)
	}
	if !(applePos < mangoPos && mangoPos < zebraPos) {
		t.Errorf("foreign keys not sorted: apple@%d mango@%d zebra@%d\noutput: %q",
			applePos, mangoPos, zebraPos, data)
	}
}

func TestReadSettings_missing_file_returns_empty(t *testing.T) {
	fs := afero.NewMemMapFs()
	s, err := readSettings(fs, "/nonexistent/settings.json")
	if err != nil {
		t.Fatalf("readSettings on missing file returned error: %v", err)
	}
	if s.StatusLine != nil {
		t.Errorf("expected nil StatusLine for missing file, got %v", s.StatusLine)
	}
	if len(s.Hooks) != 0 {
		t.Errorf("expected empty Hooks for missing file, got %v", s.Hooks)
	}
}
