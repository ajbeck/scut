package hook

import "go/format"

// formatGo formats Go source using gofmt rules.
// Returns nil, nil if src has syntax errors (decline to format).
func formatGo(src []byte) ([]byte, error) {
	formatted, err := format.Source(src)
	if err != nil {
		return nil, nil //nolint:nilerr // syntax errors are not our problem
	}
	return formatted, nil
}
