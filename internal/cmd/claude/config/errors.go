//go:build goexperiment.jsonv2

package config

import "errors"

// ErrForeignStatusLine is returned by install when settings.json already
// has a statusLine entry whose command does not start with "botctrl ".
// Callers must either remove the foreign entry manually or pass --only
// excluding status-line.
var ErrForeignStatusLine = errors.New("settings.json has a non-botctrl statusLine")

// ErrUnknownOnlyToken is returned when --only contains a token that is
// neither a registered hook slug nor the literal "status-line". The wrapped
// error message lists every valid token alphabetically.
var ErrUnknownOnlyToken = errors.New("unknown --only token")
