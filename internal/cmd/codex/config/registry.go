//go:build goexperiment.jsonv2

package config

import (
	"fmt"
	"sort"
	"strings"
)

// hookSpec describes one installable Codex hook command.
type hookSpec struct {
	Slug          string
	Event         string
	Matcher       string
	StatusMessage string
}

// hookSpecs is the registry of supported Codex hook commands.
//
// Slugs must exactly match the cmd:"" tag values on fields of hook.Cmd.
var hookSpecs = []hookSpec{
	{Slug: "session-start", Event: "SessionStart", Matcher: "startup|resume"},
	{Slug: "subagent-start", Event: "SubagentStart", Matcher: "*"},
	{Slug: "pre-tool-use", Event: "PreToolUse", Matcher: "*"},
	{Slug: "permission-request", Event: "PermissionRequest", Matcher: "*"},
	{Slug: "post-tool-use", Event: "PostToolUse", Matcher: "apply_patch|Edit|Write", StatusMessage: "Formatting..."},
	{Slug: "pre-compact", Event: "PreCompact", Matcher: "*"},
	{Slug: "post-compact", Event: "PostCompact", Matcher: "*"},
	{Slug: "user-prompt-submit", Event: "UserPromptSubmit", Matcher: "*"},
	{Slug: "subagent-stop", Event: "SubagentStop", Matcher: "*"},
	{Slug: "stop", Event: "Stop", Matcher: "*"},
}

// defaultInstallSlugs are installed when --only is omitted.
var defaultInstallSlugs = []string{"post-tool-use"}

func hookSpecBySlug(slug string) (hookSpec, bool) {
	for _, s := range hookSpecs {
		if s.Slug == slug {
			return s, true
		}
	}
	return hookSpec{}, false
}

func resolveInstallSet(only []string) (map[string]bool, error) {
	if len(only) == 0 {
		set := make(map[string]bool, len(defaultInstallSlugs))
		for _, slug := range defaultInstallSlugs {
			set[slug] = true
		}
		return set, nil
	}

	valid := validTokenSet()
	set := make(map[string]bool, len(only))
	for _, tok := range only {
		if !valid[tok] {
			return nil, fmt.Errorf("%w %q; valid tokens: %s",
				ErrUnknownOnlyToken, tok, strings.Join(sortedValidTokens(), ", "))
		}
		set[tok] = true
	}
	return set, nil
}

func resolveRemoveSet(only []string) (map[string]bool, error) {
	if len(only) == 0 {
		set := make(map[string]bool, len(hookSpecs))
		for _, s := range hookSpecs {
			set[s.Slug] = true
		}
		return set, nil
	}
	return resolveInstallSet(only)
}

func validTokenSet() map[string]bool {
	m := make(map[string]bool, len(hookSpecs))
	for _, s := range hookSpecs {
		m[s.Slug] = true
	}
	return m
}

func sortedValidTokens() []string {
	m := validTokenSet()
	tokens := make([]string, 0, len(m))
	for t := range m {
		tokens = append(tokens, t)
	}
	sort.Strings(tokens)
	return tokens
}
