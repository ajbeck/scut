// Package format provides code formatting utilities.
package format

import (
	"bytes"

	"go/format"

	prettier "github.com/ajbeck/goldmark-prettier-markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// FormatGo formats Go source using gofmt rules.
// Returns nil, nil if src has syntax errors (decline to format).
func FormatGo(src []byte) ([]byte, error) {
	formatted, err := format.Source(src)
	if err != nil {
		return nil, nil
	}
	return formatted, nil
}

// FormatMarkdown formats markdown source using goldmark-prettier-markdown.
// Returns nil, nil if the source cannot be parsed (decline to format).
func FormatMarkdown(src []byte) ([]byte, error) {
	md := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithParagraphTransformers(
				util.Prioritized(extension.NewTableParagraphTransformer(), 200),
			),
			parser.WithInlineParsers(
				util.Prioritized(extension.NewStrikethroughParser(), 500),
				util.Prioritized(extension.NewTaskCheckBoxParser(), 10),
				util.Prioritized(extension.NewFootnoteParser(), 101),
			),
			parser.WithBlockParsers(
				util.Prioritized(extension.NewFootnoteBlockParser(), 999),
				util.Prioritized(extension.NewDefinitionListParser(), 100),
			),
			parser.WithASTTransformers(
				util.Prioritized(extension.NewFootnoteASTTransformer(), 999),
			),
		),
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
		return nil, nil
	}
	return buf.Bytes(), nil
}
