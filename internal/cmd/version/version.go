// Package version implements the "version" command.
package version

import (
	"fmt"
	"io"

	versionmeta "github.com/ajbeck/scut/internal/version"
)

// Cmd is the Kong command for "scut version".
type Cmd struct{}

func (c *Cmd) Run(stdout io.Writer) error {
	_, err := fmt.Fprintln(stdout, versionmeta.String())
	return err
}
