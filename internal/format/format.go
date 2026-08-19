//go:build goexperiment.jsonv2

// Package format provides code formatting utilities.
package format

import (
	"bytes"

	"encoding/json/jsontext"
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
// Leading Hugo YAML, TOML, and JSON front matter is preserved verbatim.
// Returns nil, nil if the source cannot be parsed safely (decline to format).
func FormatMarkdown(src []byte) ([]byte, error) {
	frontMatter, body, ok := splitFrontMatter(src)
	if !ok {
		return nil, nil
	}

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
	if err := md.Convert(body, &buf); err != nil {
		return nil, nil
	}
	formatted := buf.Bytes()
	if len(frontMatter) == 0 {
		return formatted, nil
	}

	result := make([]byte, 0, len(frontMatter)+len(formatted))
	result = append(result, frontMatter...)
	return append(result, formatted...), nil
}

var (
	byteOrderMark = []byte{0xef, 0xbb, 0xbf}
	yamlDelimiter = []byte("---")
	tomlDelimiter = []byte("+++")
)

func splitFrontMatter(src []byte) (frontMatter, body []byte, ok bool) {
	start := skipLeadingWhitespace(src)
	if start == len(src) {
		return nil, src, true
	}

	if _, isYAML := delimiterLine(src, start, yamlDelimiter); isYAML {
		return splitDelimitedFrontMatter(src, start, yamlDelimiter)
	}
	if _, isTOML := delimiterLine(src, start, tomlDelimiter); isTOML {
		return splitDelimitedFrontMatter(src, start, tomlDelimiter)
	}
	if src[start] == '{' {
		return splitJSONFrontMatter(src, start)
	}
	return nil, src, true
}

func splitDelimitedFrontMatter(src []byte, start int, delimiter []byte) (frontMatter, body []byte, ok bool) {
	pos, _ := delimiterLine(src, start, delimiter)
	for pos < len(src) {
		if next, found := delimiterLine(src, pos, delimiter); found {
			return src[:next], src[next:], true
		}
		_, pos = nextLine(src, pos)
	}
	return nil, nil, false
}

func splitJSONFrontMatter(src []byte, start int) (frontMatter, body []byte, ok bool) {
	decoder := jsontext.NewDecoder(bytes.NewReader(src[start:]))
	value, err := decoder.ReadValue()
	if err != nil || len(value) == 0 || value[0] != '{' {
		return nil, nil, false
	}

	end := start + int(decoder.InputOffset())
	if end == len(src) {
		return src, nil, true
	}
	if src[end] == '\n' {
		end++
	} else if end+1 < len(src) && src[end] == '\r' && src[end+1] == '\n' {
		end += 2
	} else {
		return nil, nil, false
	}
	return src[:end], src[end:], true
}

func skipLeadingWhitespace(src []byte) int {
	for pos := 0; pos < len(src); {
		if bytes.HasPrefix(src[pos:], byteOrderMark) {
			pos += len(byteOrderMark)
			continue
		}
		switch src[pos] {
		case ' ', '\t', '\n', '\r', '\v', '\f':
			pos++
		default:
			return pos
		}
	}
	return len(src)
}

func delimiterLine(src []byte, start int, delimiter []byte) (int, bool) {
	line, next := nextLine(src, start)
	return next, bytes.Equal(line, delimiter)
}

func nextLine(src []byte, start int) ([]byte, int) {
	end := bytes.IndexByte(src[start:], '\n')
	if end < 0 {
		return bytes.TrimSuffix(src[start:], []byte("\r")), len(src)
	}
	end += start
	return bytes.TrimSuffix(src[start:end], []byte("\r")), end + 1
}
