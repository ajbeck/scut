package godoc

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
)

const (
	defaultGOPROXY = "https://proxy.golang.org,direct"
	defaultGOSUMDB = "sum.golang.org"
	defaultGOAUTH  = "netrc"
)

// ModuleDownloadPolicy is the Go environment policy that controls remote
// module discovery, transport, authentication, and verification.
type ModuleDownloadPolicy struct {
	GOPROXY    string
	GOPRIVATE  string
	GONOPROXY  string
	GONOSUMDB  string
	GOSUMDB    string
	GOINSECURE string
	GOAUTH     string
	GOVCS      string
}

type goEnvLoader struct {
	Lookup        func(string) (string, bool)
	ReadFile      func(string) ([]byte, error)
	UserConfigDir func() (string, error)
	GOROOT        string
}

func loadModuleDownloadPolicy() ModuleDownloadPolicy {
	return newGoEnvLoader().loadPolicy()
}

func newGoEnvLoader() goEnvLoader {
	return goEnvLoader{
		Lookup:        os.LookupEnv,
		ReadFile:      os.ReadFile,
		UserConfigDir: os.UserConfigDir,
		GOROOT:        runtime.GOROOT(),
	}
}

func (l goEnvLoader) loadPolicy() ModuleDownloadPolicy {
	values := l.loadFiles()
	get := func(name, fallback string) string {
		if value, ok := l.lookup()(name); ok && value != "" {
			return value
		}
		if value := values[name]; value != "" {
			return value
		}
		return fallback
	}

	private := get("GOPRIVATE", "")
	return ModuleDownloadPolicy{
		GOPROXY:    get("GOPROXY", defaultGOPROXY),
		GOPRIVATE:  private,
		GONOPROXY:  get("GONOPROXY", private),
		GONOSUMDB:  get("GONOSUMDB", private),
		GOSUMDB:    get("GOSUMDB", defaultGOSUMDB),
		GOINSECURE: get("GOINSECURE", ""),
		GOAUTH:     get("GOAUTH", defaultGOAUTH),
		GOVCS:      get("GOVCS", ""),
	}
}

func (l goEnvLoader) loadFiles() map[string]string {
	values := make(map[string]string)
	if userFile := l.userEnvFile(); userFile != "" {
		l.readEnvFile(values, userFile, false)
	}
	goroot := l.GOROOT
	if configured := values["GOROOT"]; configured != "" {
		goroot = configured
	}
	if configured, ok := l.lookup()("GOROOT"); ok && configured != "" {
		goroot = configured
	}
	if goroot != "" {
		l.readEnvFile(values, filepath.Join(goroot, "go.env"), true)
	}
	return values
}

func (l goEnvLoader) userEnvFile() string {
	if value, ok := l.lookup()("GOENV"); ok && value != "" {
		if value == "off" {
			return ""
		}
		return value
	}
	dir, err := l.userConfigDir()()
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, "go", "env")
}

func (l goEnvLoader) readEnvFile(values map[string]string, name string, preserve bool) {
	data, err := l.readFile()(name)
	if err != nil {
		return
	}
	for len(data) > 0 {
		line := data
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			line, data = data[:i], data[i+1:]
		} else {
			data = nil
		}
		i := bytes.IndexByte(line, '=')
		if i < 1 || line[0] < 'A' || line[0] > 'Z' {
			continue
		}
		key := string(line[:i])
		if preserve {
			if _, ok := values[key]; ok {
				continue
			}
		}
		values[key] = string(line[i+1:])
	}
}

func (l goEnvLoader) lookup() func(string) (string, bool) {
	if l.Lookup != nil {
		return l.Lookup
	}
	return func(string) (string, bool) { return "", false }
}

func (l goEnvLoader) readFile() func(string) ([]byte, error) {
	if l.ReadFile != nil {
		return l.ReadFile
	}
	return os.ReadFile
}

func (l goEnvLoader) userConfigDir() func() (string, error) {
	if l.UserConfigDir != nil {
		return l.UserConfigDir
	}
	return os.UserConfigDir
}
