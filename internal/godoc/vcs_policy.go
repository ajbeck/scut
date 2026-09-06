package godoc

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/mod/module"
)

type vcsRule struct {
	Pattern string
	Allowed []string
}

func checkGitAllowed(policy ModuleDownloadPolicy, modulePath string) error {
	rules, err := parseVCSRules(policy.GOVCS)
	if err != nil {
		return err
	}
	rules = append(rules,
		vcsRule{Pattern: "private", Allowed: []string{"all"}},
		vcsRule{Pattern: "public", Allowed: []string{"git", "hg"}},
	)
	private := matchesModulePattern(policy.GOPRIVATE, modulePath)
	for _, rule := range rules {
		matched := false
		switch rule.Pattern {
		case "private":
			matched = private
		case "public":
			matched = !private
		default:
			matched = module.MatchPrefixPatterns(rule.Pattern, modulePath)
		}
		if !matched {
			continue
		}
		for _, allowed := range rule.Allowed {
			if allowed == "git" || allowed == "all" {
				return nil
			}
		}
		kind := "public"
		if private {
			kind = "private"
		}
		return fmt.Errorf("GOVCS disallows using git for %s module %s", kind, modulePath)
	}
	return errors.New("GOVCS has no rule for module")
}

func parseVCSRules(value string) ([]vcsRule, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	seen := make(map[string]string)
	var rules []vcsRule
	for entry := range strings.SplitSeq(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return nil, errors.New("empty entry in GOVCS")
		}
		pattern, list, ok := strings.Cut(entry, ":")
		if !ok {
			return nil, fmt.Errorf("malformed entry in GOVCS (missing colon): %q", entry)
		}
		pattern, list = strings.TrimSpace(pattern), strings.TrimSpace(list)
		if pattern == "" || list == "" {
			return nil, fmt.Errorf("malformed entry in GOVCS: %q", entry)
		}
		if previous := seen[pattern]; previous != "" {
			return nil, fmt.Errorf("unreachable pattern in GOVCS: %q after %q", entry, previous)
		}
		seen[pattern] = entry
		allowed := strings.Split(list, "|")
		for i := range allowed {
			allowed[i] = strings.TrimSpace(allowed[i])
			if allowed[i] == "" {
				return nil, fmt.Errorf("empty VCS name in GOVCS: %q", entry)
			}
		}
		rules = append(rules, vcsRule{Pattern: pattern, Allowed: allowed})
	}
	return rules, nil
}
