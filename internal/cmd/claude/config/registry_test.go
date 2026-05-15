//go:build goexperiment.jsonv2

package config

import (
	"reflect"
	"testing"

	"github.com/ajbeck/botctrl/internal/cmd/claude/hook"
)

// TestHookSpecsMatchHookCmd verifies that hookSpecs and hook.Cmd define exactly
// the same set of event slugs — no orphans on either side.
func TestHookSpecsMatchHookCmd(t *testing.T) {
	// Collect slugs from hook.Cmd via reflection.
	hookCmdType := reflect.TypeOf(hook.Cmd{})
	cmdSlugs := make(map[string]bool, hookCmdType.NumField())
	for i := range hookCmdType.NumField() {
		slug := hookCmdType.Field(i).Tag.Get("cmd")
		if slug == "" {
			continue
		}
		cmdSlugs[slug] = true
	}

	// Collect slugs from hookSpecs.
	specSlugs := make(map[string]bool, len(hookSpecs))
	for _, s := range hookSpecs {
		specSlugs[s.Slug] = true
	}

	// Every hook.Cmd slug must appear in hookSpecs.
	for slug := range cmdSlugs {
		if !specSlugs[slug] {
			t.Errorf("hook.Cmd has slug %q that is missing from hookSpecs", slug)
		}
	}

	// Every hookSpec slug must appear in hook.Cmd.
	for slug := range specSlugs {
		if !cmdSlugs[slug] {
			t.Errorf("hookSpecs has slug %q that has no matching hook.Cmd field", slug)
		}
	}

	// Counts must match (catches duplicates).
	if len(cmdSlugs) != len(hookSpecs) {
		t.Errorf("hook.Cmd has %d slugs but hookSpecs has %d entries", len(cmdSlugs), len(hookSpecs))
	}
}
