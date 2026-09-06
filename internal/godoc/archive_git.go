package godoc

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	"golang.org/x/mod/module"
	modzip "golang.org/x/mod/zip"
)

func moduleArchiveFromFS(mod module.Version, fs afero.Fs, revision string) (ModuleArchive, error) {
	return moduleArchiveFromFSRoot(mod, fs, "", revision)
}

func moduleArchiveFromFSRoot(mod module.Version, fs afero.Fs, root, revision string) (ModuleArchive, error) {
	files, err := moduleFilesFromFSRoot(fs, root)
	if err != nil {
		return ModuleArchive{}, err
	}
	var archive bytes.Buffer
	if err := modzip.Create(&archive, mod, files); err != nil {
		return ModuleArchive{}, err
	}
	return ModuleArchive{Module: mod, Data: archive.Bytes(), Revision: revision}, nil
}

func moduleFilesFromFS(fs afero.Fs) ([]modzip.File, error) {
	return moduleFilesFromFSRoot(fs, "")
}

func moduleFilesFromFSRoot(fs afero.Fs, root string) ([]modzip.File, error) {
	var files []modzip.File
	walkRoot := filepath.Join("/", filepath.FromSlash(root))
	err := afero.Walk(fs, walkRoot, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if name == walkRoot {
			return nil
		}
		if info.IsDir() {
			if isVCSDirectory(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(walkRoot, name)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		files = append(files, aferoModuleFile{fs: fs, name: name, path: rel, info: info})
		return nil
	})
	return files, err
}

func isVCSDirectory(name string) bool {
	switch name {
	case ".bzr", ".git", ".hg", ".svn":
		return true
	default:
		return false
	}
}

type aferoModuleFile struct {
	fs   afero.Fs
	name string
	path string
	info os.FileInfo
}

func (f aferoModuleFile) Path() string {
	return f.path
}

func (f aferoModuleFile) Lstat() (os.FileInfo, error) {
	return f.info, nil
}

func (f aferoModuleFile) Open() (io.ReadCloser, error) {
	return f.fs.Open(f.name)
}
