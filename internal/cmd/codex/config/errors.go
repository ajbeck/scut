//go:build goexperiment.jsonv2

package config

import "errors"

// ErrUnknownOnlyToken is returned when --only contains a token that is
// not a known Codex hook slug.
var ErrUnknownOnlyToken = errors.New("unknown --only token")
