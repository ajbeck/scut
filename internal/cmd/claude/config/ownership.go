//go:build goexperiment.jsonv2

package config

import "strings"

// owns reports whether command is a scut invocation we should manage.
// First scope: exact "scut" or "scut " / "scut\t" prefix on the leading token.
// Leading whitespace is stripped before the check; only the first token matters.
func owns(command string) bool {
	c := strings.TrimLeft(command, " \t")
	if c == "scut" {
		return true
	}
	return strings.HasPrefix(c, "scut ") || strings.HasPrefix(c, "scut\t")
}
