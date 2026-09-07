// Package format provides code formatting utilities.
package format

import (
	"bytes"

	"encoding/json/jsontext"
	"go/format"

	prettier "github.com/ajbeck/goldmark-prettier-markdown/v2"
	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
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

	p := parser.New(
		parser.WithExtensions(
			extension.NewTableParser(),
			extension.NewStrikethroughParser(),
			extension.NewTaskListItemParser(),
			extension.NewFootnoteParser(),
			extension.NewDefinitionListParser(),
		),
	)
	document := p.Parse(body)

	var buf bytes.Buffer
	r := prettier.NewRenderer(prettier.WithProseWrap(prettier.ProseWrapPreserve))
	if err := r.Render(&buf, body, document); err != nil {
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
