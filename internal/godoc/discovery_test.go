package godoc

import "testing"

func TestParseGoImportMetaAcceptsRepositorySubdirectory(t *testing.T) {
	meta, err := parseGoImportMeta(
		`<meta name="go-import" content="example.com/mod git https://git.example/repo.git nested/root">`,
		"example.com/mod/pkg",
	)
	if err != nil {
		t.Fatalf("parseGoImportMeta() error = %v", err)
	}
	if meta.Prefix != "example.com/mod" || meta.Repo != "https://git.example/repo.git" || meta.SubDir != "nested/root" {
		t.Fatalf("meta = %#v", meta)
	}
}
