// Package formatignore matches formatter ignore files against paths.
package formatignore

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/spf13/afero"
)

const (
	gitDir         = ".git"
	prettierIgnore = ".prettierignore"
	scutIgnore     = ".scutignore"
)

// Matcher matches file paths against formatter ignore files.
type Matcher struct {
	root    string
	matcher gitignore.Matcher
}

// ForPath returns a matcher rooted at the nearest formatter root for path.
//
// The root is the first parent directory containing .git, .prettierignore, or
// .scutignore. If no root is found, the returned bool is false.
func ForPath(fs afero.Fs, path string) (Matcher, bool, error) {
	root, ok, err := findRoot(fs, filepath.Dir(path))
	if err != nil || !ok {
		return Matcher{}, false, err
	}

	patterns, err := readPatterns(fs, root)
	if err != nil {
		return Matcher{}, false, err
	}
	return Matcher{root: root, matcher: gitignore.NewMatcher(patterns)}, true, nil
}

// Match reports whether path is ignored.
func (m Matcher) Match(path string, isDir bool) (bool, error) {
	rel, err := filepath.Rel(m.root, path)
	if err != nil {
		return false, err
	}
	if rel == "." {
		return false, nil
	}
	return m.matcher.Match(splitPath(rel), isDir), nil
}

// MatchPath reports whether path is ignored by the nearest formatter root.
func MatchPath(fs afero.Fs, path string, isDir bool) (bool, error) {
	m, ok, err := ForPath(fs, path)
	if err != nil || !ok {
		return false, err
	}
	return m.Match(path, isDir)
}

func findRoot(fs afero.Fs, dir string) (string, bool, error) {
	dir = filepath.Clean(dir)
	for {
		ok, err := isRoot(fs, dir)
		if err != nil || ok {
			return dir, ok, err
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false, nil
		}
		dir = parent
	}
}

func isRoot(fs afero.Fs, dir string) (bool, error) {
	for _, name := range []string{gitDir, prettierIgnore, scutIgnore} {
		_, err := fs.Stat(filepath.Join(dir, name))
		if err == nil {
			return true, nil
		}
		if !isNotExist(err) {
			return false, err
		}
	}
	return false, nil
}

func readPatterns(fs afero.Fs, root string) ([]gitignore.Pattern, error) {
	var patterns []gitignore.Pattern
	for _, name := range []string{prettierIgnore, scutIgnore} {
		ps, err := readPatternFile(fs, filepath.Join(root, name))
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, ps...)
	}
	return patterns, nil
}

func readPatternFile(fs afero.Fs, path string) (patterns []gitignore.Pattern, err error) {
	f, err := fs.Open(path)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() {
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || len(strings.TrimSpace(line)) == 0 {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(line, nil))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return patterns, nil
}

func splitPath(path string) []string {
	path = filepath.ToSlash(filepath.Clean(path))
	return strings.Split(path, "/")
}

func isNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
