package godoc

import (
	"strings"
	"testing"
)

func TestCheckGitAllowedUsesFirstMatchingRuleAndDefaults(t *testing.T) {
	tests := []struct {
		name      string
		policy    ModuleDownloadPolicy
		module    string
		wantError bool
	}{
		{name: "public default", module: "example.com/public/mod"},
		{name: "private default", policy: ModuleDownloadPolicy{GOPRIVATE: "*.corp.example"}, module: "repo.corp.example/mod"},
		{name: "public denied", policy: ModuleDownloadPolicy{GOVCS: "public:off"}, module: "example.com/public/mod", wantError: true},
		{name: "private denied", policy: ModuleDownloadPolicy{GOPRIVATE: "*.corp.example", GOVCS: "private:off"}, module: "repo.corp.example/mod", wantError: true},
		{name: "specific overrides public", policy: ModuleDownloadPolicy{GOVCS: "example.com:off,public:git"}, module: "example.com/mod", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkGitAllowed(tt.policy, tt.module)
			if (err != nil) != tt.wantError {
				t.Fatalf("checkGitAllowed() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestParseVCSRulesRejectsDuplicatePattern(t *testing.T) {
	_, err := parseVCSRules("public:git,public:off")
	if err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("parseVCSRules() error = %v", err)
	}
}
