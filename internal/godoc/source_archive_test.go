package godoc

import (
	"testing"

	"golang.org/x/mod/module"
)

func TestArchiveFetcherUsesBuildListSelectionBeforeLatestAlias(t *testing.T) {
	const modPath = "example.com/acme/tool"
	selected := module.Version{Path: modPath, Version: "v1.2.3"}
	latest := module.Version{Path: modPath, Version: "v1.9.0"}
	store := FileArchiveStore{Root: t.TempDir()}
	for _, mod := range []module.Version{selected, latest} {
		archive := ModuleArchive{
			Module: mod,
			Data: moduleZip(t, mod.Path, mod.Version, map[string]string{
				"tool.go": "package tool\n\nconst Version = \"" + mod.Version + "\"\n",
			}),
		}
		if err := store.Put(t.Context(), archive); err != nil {
			t.Fatalf("Put(%s) error = %v", mod.Version, err)
		}
	}
	if err := store.SetLatest(t.Context(), latest); err != nil {
		t.Fatalf("SetLatest() error = %v", err)
	}

	source, err := (ArchiveFetcher{
		Store:    store,
		Selector: fixedModuleSelector{selection: ModuleSelection{Module: selected, Source: selected}},
	}).Fetch(t.Context(), modPath, Options{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got, want := source.Module.Version, selected.Version; got != want {
		t.Fatalf("Module.Version = %q, want selected version %q", got, want)
	}
	if got, want := string(source.Files[0].Data), "package tool\n\nconst Version = \"v1.2.3\"\n"; got != want {
		t.Fatalf("source data = %q, want %q", got, want)
	}
}
