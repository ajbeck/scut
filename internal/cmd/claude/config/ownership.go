//go:build goexperiment.jsonv2

package config

import "strings"

// owns reports whether command is a botctrl invocation we should manage.
// First scope: exact "botctrl" or "botctrl " / "botctrl\t" prefix on the leading token.
// Leading whitespace is stripped before the check; only the first token matters.
func owns(command string) bool {
	c := strings.TrimLeft(command, " \t")
	if c == "botctrl" {
		return true
	}
	return strings.HasPrefix(c, "botctrl ") || strings.HasPrefix(c, "botctrl\t")
}
