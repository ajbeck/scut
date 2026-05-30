//go:build goexperiment.jsonv2

package config

import "strings"

func owns(command string) bool {
	c := strings.TrimLeft(command, " \t")
	return strings.HasPrefix(c, "scut codex hook ") ||
		strings.HasPrefix(c, "scut codex --log hook ") ||
		strings.HasPrefix(c, "scut codex --log-level=")
}

func isScutGroup(g HookGroup) bool {
	if len(g.Hooks) == 0 {
		return false
	}
	for _, h := range g.Hooks {
		if h.Type != "command" || !owns(h.Command) {
			return false
		}
	}
	return true
}

func mergeHookGroup(groups []HookGroup, next HookGroup) []HookGroup {
	for i, existing := range groups {
		if existing.Matcher != next.Matcher {
			continue
		}
		if isScutGroup(existing) {
			groups[i] = next
			return groups
		}
	}
	return append(groups, next)
}
