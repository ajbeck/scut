package godoc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb"
	"golang.org/x/mod/sumdb/note"
)

const publicGoChecksumKey = "sum.golang.org+033de0ae+Ac4zctda0e5eza+HJyk9SxEdh+s3Ux18htTTAD8OuAn8"

// GoChecksumDatabase verifies module sums using the checksum database selected
// by Go's environment policy.
type GoChecksumDatabase struct {
	Policy        ModuleDownloadPolicy
	Client        *http.Client
	Authenticator *GoAuthenticator
	StateDir      string
}

func (d GoChecksumDatabase) Lookup(ctx context.Context, mod module.Version) (string, []string, error) {
	name, key, direct, base, err := parseChecksumDatabase(d.Policy.GOSUMDB)
	if err != nil {
		return "", nil, err
	}
	ops := &checksumClientOps{
		ctx:           ctx,
		key:           key,
		name:          name,
		direct:        direct,
		base:          base,
		policy:        d.Policy,
		client:        d.Client,
		authenticator: d.Authenticator,
		stateDir:      d.StateDir,
		cache:         make(map[string][]byte),
		config:        make(map[string][]byte),
	}
	client := sumdb.NewClient(ops)
	client.SetGONOSUMDB(d.Policy.GONOSUMDB)
	lines, err := client.Lookup(mod.Path, mod.Version)
	if err != nil {
		if message := ops.securityMessage(); errors.Is(err, sumdb.ErrSecurity) && message != "" {
			return name, nil, errors.New(message)
		}
		return name, nil, err
	}
	return name, lines, nil
}

// DefaultChecksumStateDir returns scut's persistent checksum transparency
// state. It is deliberately separate from the disposable module cache.
func DefaultChecksumStateDir() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving user configuration directory: %w", err)
	}
	return filepath.Join(root, "scut", "gotools", "sumdb"), nil
}

func parseChecksumDatabase(value string) (name, key string, direct, base *url.URL, err error) {
	if value == "sum.golang.google.cn" {
		value = "sum.golang.org https://sum.golang.google.cn"
	}
	if value == "off" {
		return "", "", nil, nil, errors.New("checksum database disabled by GOSUMDB=off")
	}
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return "", "", nil, nil, errors.New("missing GOSUMDB")
	}
	if len(fields) > 2 {
		return "", "", nil, nil, errors.New("invalid GOSUMDB: too many fields")
	}
	key = fields[0]
	if key == "sum.golang.org" {
		key = publicGoChecksumKey
	}
	verifier, err := note.NewVerifier(key)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("invalid GOSUMDB: %w", err)
	}
	name = verifier.Name()
	direct, err = url.Parse("https://" + name)
	if err != nil || strings.HasSuffix(name, "/") || direct.Host == "" || direct.RawPath != "" ||
		*direct != (url.URL{Scheme: "https", Host: direct.Host, Path: direct.Path}) {
		return "", "", nil, nil, fmt.Errorf("invalid checksum database name %q", name)
	}
	if len(fields) == 2 {
		base, err = url.Parse(fields[1])
		if err != nil {
			return "", "", nil, nil, fmt.Errorf("invalid GOSUMDB URL: %w", err)
		}
		if base.Scheme != "https" && base.Scheme != "http" && base.Scheme != "file" {
			return "", "", nil, nil, fmt.Errorf("invalid GOSUMDB URL scheme %q", base.Scheme)
		}
	}
	return name, key, direct, base, nil
}

type checksumClientOps struct {
	ctx           context.Context
	key           string
	name          string
	direct        *url.URL
	policy        ModuleDownloadPolicy
	client        *http.Client
	authenticator *GoAuthenticator
	stateDir      string

	baseOnce sync.Once
	base     *url.URL
	baseErr  error

	mu            sync.Mutex
	cache         map[string][]byte
	config        map[string][]byte
	securityError string
}

func (c *checksumClientOps) ReadRemote(name string) ([]byte, error) {
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}
	c.baseOnce.Do(c.initBase)
	if c.baseErr != nil {
		return nil, c.baseErr
	}
	return c.readURL(joinChecksumURL(c.base, name))
}

func (c *checksumClientOps) initBase() {
	if c.base != nil {
		return
	}
	routes, err := parseProxyRoutes(c.policy)
	if err != nil {
		c.baseErr = err
		return
	}
	for _, route := range routes {
		switch route.Kind {
		case proxyRouteNoProxy:
			continue
		case proxyRouteDirect, proxyRouteOff:
			c.base = c.direct
			return
		case proxyRouteURL:
			proxy, parseErr := url.Parse(route.URL)
			if parseErr != nil {
				c.baseErr = parseErr
				return
			}
			base := joinChecksumURL(proxy, "/sumdb/"+c.name)
			_, probeErr := c.readURL(joinChecksumURL(base, "/supported"))
			if probeErr == nil {
				c.base = base
				return
			}
			if !route.FallbackOnError && !errors.Is(probeErr, fs.ErrNotExist) {
				c.baseErr = probeErr
				return
			}
		}
	}
	// Go connects directly to the checksum database when no configured module
	// proxy advertises checksum-database proxying.
	c.base = c.direct
}

func (c *checksumClientOps) readURL(target *url.URL) ([]byte, error) {
	if target == nil {
		return nil, errors.New("checksum database URL is unavailable")
	}
	if target.Scheme == "file" {
		data, err := os.ReadFile(filepath.FromSlash(target.Path))
		if errors.Is(err, os.ErrNotExist) {
			return nil, checksumHTTPError{Status: http.StatusNotFound, URL: target.String()}
		}
		return data, err
	}
	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	var client httpDoer
	if c.client == nil {
		client = http.DefaultClient
	} else {
		client = c.client
	}
	client = c.authenticator.Client(client)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, checksumHTTPError{Status: resp.StatusCode, URL: target.Redacted()}
	}
	return io.ReadAll(resp.Body)
}

func (c *checksumClientOps) ReadConfig(name string) ([]byte, error) {
	if name == "key" {
		return []byte(c.key), nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stateDir == "" {
		return bytes.Clone(c.config[name]), nil
	}
	path, err := checksumStatePath(c.stateDir, name)
	if err != nil {
		return nil, err
	}
	unlock, err := acquireChecksumStateLock(c.ctx, path)
	if err != nil {
		return nil, err
	}
	defer unlock()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []byte{}, nil
	}
	return data, err
}

func (c *checksumClientOps) WriteConfig(name string, old, newValue []byte) error {
	if name == "key" {
		return errors.New("cannot write checksum database key")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stateDir == "" {
		if !bytes.Equal(c.config[name], old) {
			return sumdb.ErrWriteConflict
		}
		c.config[name] = bytes.Clone(newValue)
		return nil
	}
	path, err := checksumStatePath(c.stateDir, name)
	if err != nil {
		return err
	}
	unlock, err := acquireChecksumStateLock(c.ctx, path)
	if err != nil {
		return err
	}
	defer unlock()
	current, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		current = nil
	} else if err != nil {
		return err
	}
	if !bytes.Equal(current, old) {
		return sumdb.ErrWriteConflict
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	if err := file.Truncate(int64(len(newValue))); err != nil {
		return err
	}
	if _, err := file.WriteAt(newValue, 0); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func (c *checksumClientOps) ReadCache(name string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	data, ok := c.cache[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return bytes.Clone(data), nil
}

func (c *checksumClientOps) WriteCache(name string, data []byte) {
	c.mu.Lock()
	c.cache[name] = bytes.Clone(data)
	c.mu.Unlock()
}

func (*checksumClientOps) Log(string) {}

func (c *checksumClientOps) SecurityError(message string) {
	c.mu.Lock()
	c.securityError = message
	c.mu.Unlock()
}

func (c *checksumClientOps) securityMessage() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.securityError
}

func checksumStatePath(root, name string) (string, error) {
	name = filepath.FromSlash(name)
	if !filepath.IsLocal(name) {
		return "", fmt.Errorf("invalid checksum state path %q", name)
	}
	return filepath.Join(root, name), nil
}

func acquireChecksumStateLock(ctx context.Context, statePath string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(statePath), 0o700); err != nil {
		return nil, err
	}
	lockPath := statePath + ".lock"
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		err := os.Mkdir(lockPath, 0o700)
		if err == nil {
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > time.Minute {
			_ = os.Remove(lockPath)
			continue
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func joinChecksumURL(base *url.URL, suffix string) *url.URL {
	if base == nil {
		return nil
	}
	target := *base
	target.Path = strings.TrimRight(base.Path, "/") + "/" + strings.TrimLeft(suffix, "/")
	target.RawPath = ""
	return &target
}

type checksumHTTPError struct {
	Status int
	URL    string
}

func (e checksumHTTPError) Error() string {
	return fmt.Sprintf("checksum database request %s returned %s", e.URL, http.StatusText(e.Status))
}

func (e checksumHTTPError) Is(target error) bool {
	return target == fs.ErrNotExist && (e.Status == http.StatusNotFound || e.Status == http.StatusGone)
}

var _ ChecksumDatabase = GoChecksumDatabase{}
var _ sumdb.ClientOps = (*checksumClientOps)(nil)
