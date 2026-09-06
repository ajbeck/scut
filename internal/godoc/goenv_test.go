package godoc

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestGoEnvLoaderUsesGoPrecedenceAndDefaults(t *testing.T) {
	userConfig := filepath.Join("test", "config")
	goroot := filepath.Join("test", "goroot")
	files := map[string][]byte{
		filepath.Join(userConfig, "go", "env"): []byte("GOPROXY=https://user.example,direct\nGOPRIVATE=*.corp.example\nGOAUTH=off\n"),
		filepath.Join(goroot, "go.env"):        []byte("GOPROXY=https://proxy.golang.org,direct\nGOSUMDB=sum.golang.org\nGOAUTH=netrc\n"),
	}
	loader := goEnvLoader{
		Lookup: func(name string) (string, bool) {
			if name == "GOPROXY" {
				return "https://process.example|direct", true
			}
			return "", false
		},
		ReadFile: func(name string) ([]byte, error) {
			if data, ok := files[name]; ok {
				return data, nil
			}
			return nil, errors.New("missing")
		},
		UserConfigDir: func() (string, error) { return userConfig, nil },
		GOROOT:        goroot,
	}

	got := loader.loadPolicy()
	if got.GOPROXY != "https://process.example|direct" {
		t.Fatalf("GOPROXY = %q", got.GOPROXY)
	}
	if got.GOPRIVATE != "*.corp.example" || got.GONOPROXY != got.GOPRIVATE || got.GONOSUMDB != got.GOPRIVATE {
		t.Fatalf("private policy = %#v", got)
	}
	if got.GOSUMDB != defaultGOSUMDB || got.GOAUTH != "off" {
		t.Fatalf("defaults = %#v", got)
	}
}

func TestGoEnvLoaderExplicitOverridesPrivateDefaults(t *testing.T) {
	loader := goEnvLoader{
		Lookup: func(name string) (string, bool) {
			values := map[string]string{
				"GOPRIVATE": "*.corp.example",
				"GONOPROXY": "none",
				"GONOSUMDB": "secure.corp.example",
			}
			value, ok := values[name]
			return value, ok
		},
		ReadFile:      func(string) ([]byte, error) { return nil, errors.New("missing") },
		UserConfigDir: func() (string, error) { return "", errors.New("missing") },
	}

	got := loader.loadPolicy()
	if got.GONOPROXY != "none" || got.GONOSUMDB != "secure.corp.example" {
		t.Fatalf("policy = %#v", got)
	}
	if got.GOPROXY != defaultGOPROXY || got.GOAUTH != defaultGOAUTH {
		t.Fatalf("defaults = %#v", got)
	}
}

func TestGoEnvLoaderGOENVOffSkipsUserFile(t *testing.T) {
	var reads []string
	loader := goEnvLoader{
		Lookup: func(name string) (string, bool) {
			if name == "GOENV" {
				return "off", true
			}
			return "", false
		},
		ReadFile: func(name string) ([]byte, error) {
			reads = append(reads, name)
			return []byte("GOPROXY=https://goroot.example,direct\n"), nil
		},
		UserConfigDir: func() (string, error) { return filepath.Join("test", "config"), nil },
		GOROOT:        filepath.Join("test", "goroot"),
	}

	got := loader.loadPolicy()
	if got.GOPROXY != "https://goroot.example,direct" {
		t.Fatalf("GOPROXY = %q", got.GOPROXY)
	}
	if len(reads) != 1 || reads[0] != filepath.Join(loader.GOROOT, "go.env") {
		t.Fatalf("reads = %#v", reads)
	}
}

func TestGoEnvLoaderUserFileWinsOverGOROOTFile(t *testing.T) {
	userFile := filepath.Join("test", "custom-env")
	gorootFile := filepath.Join("test", "goroot", "go.env")
	loader := goEnvLoader{
		Lookup: func(name string) (string, bool) {
			if name == "GOENV" {
				return userFile, true
			}
			return "", false
		},
		ReadFile: func(name string) ([]byte, error) {
			switch name {
			case userFile:
				return []byte("GOPROXY=https://user.example\n"), nil
			case gorootFile:
				return []byte("GOPROXY=https://goroot.example\nGOSUMDB=corp.sum.example\n"), nil
			default:
				return nil, errors.New("missing")
			}
		},
		GOROOT: filepath.Dir(gorootFile),
	}

	got := loader.loadPolicy()
	if got.GOPROXY != "https://user.example" || got.GOSUMDB != "corp.sum.example" {
		t.Fatalf("policy = %#v", got)
	}
}
