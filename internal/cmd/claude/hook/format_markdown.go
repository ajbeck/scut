package hook

import (
	"bytes"

	prettier "github.com/ajbeck/goldmark-prettier-markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// formatMarkdown formats markdown source using goldmark-prettier-markdown.
// Returns nil, nil if the source cannot be parsed (decline to format).
func formatMarkdown(src []byte) ([]byte, error) {
	md := goldmark.New(
		goldmark.WithRenderer(
			renderer.NewRenderer(
				renderer.WithNodeRenderers(
					util.Prioritized(
						prettier.NewRenderer(
							prettier.WithProseWrap(prettier.ProseWrapPreserve),
						), 1000),
				),
			),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		return nil, nil //nolint:nilerr // parse errors are not our problem
	}
	return buf.Bytes(), nil
}
