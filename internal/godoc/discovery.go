package godoc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

type discoveryFunc func(string) string

type goImportMeta struct {
	Prefix string
	VCS    string
	Repo   string
}

var metaTagPattern = regexp.MustCompile(`(?is)<meta\s+[^>]*name=["']go-import["'][^>]*>`)
var contentAttrPattern = regexp.MustCompile(`(?is)\scontent=["']([^"']+)["']`)

func discoverGoImport(ctx context.Context, client *http.Client, importPath string, discoveryURL discoveryFunc) (goImportMeta, error) {
	if discoveryURL == nil {
		discoveryURL = func(path string) string {
			return "https://" + path + "?go-get=1"
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL(importPath), nil)
	if err != nil {
		return goImportMeta{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return goImportMeta{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return goImportMeta{}, fmt.Errorf("go-import discovery returned %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return goImportMeta{}, err
	}
	return parseGoImportMeta(string(body), importPath)
}

func parseGoImportMeta(html, importPath string) (goImportMeta, error) {
	for _, tag := range metaTagPattern.FindAllString(html, -1) {
		match := contentAttrPattern.FindStringSubmatch(tag)
		if len(match) != 2 {
			continue
		}
		fields := strings.Fields(match[1])
		if len(fields) != 3 {
			continue
		}
		if importPath == fields[0] || strings.HasPrefix(importPath, fields[0]+"/") {
			return goImportMeta{Prefix: fields[0], VCS: fields[1], Repo: fields[2]}, nil
		}
	}
	return goImportMeta{}, ErrSourceNotApplicable
}
