package godoc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type goAuthCommandRunner interface {
	Run(context.Context, []string, string, *http.Response) ([]byte, error)
}

type execGoAuthCommandRunner struct{}

func (execGoAuthCommandRunner) Run(ctx context.Context, args []string, requestURL string, response *http.Response) ([]byte, error) {
	if len(args) == 0 {
		return nil, errors.New("GOAUTH command is empty")
	}
	commandArgs := slices.Clone(args[1:])
	if requestURL != "" {
		commandArgs = append(commandArgs, requestURL)
	}
	cmd := exec.CommandContext(ctx, args[0], commandArgs...)
	if response != nil {
		cmd.Stdin = strings.NewReader(formatAuthResponse(response))
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running GOAUTH command %q: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}

// GoAuthenticator applies GOAUTH credentials to HTTPS requests and retries one
// failed 4xx request after URL-specific credential discovery.
type GoAuthenticator struct {
	Config string

	Runner   goAuthCommandRunner
	ReadFile func(string) ([]byte, error)
	Lookup   func(string) (string, bool)
	HomeDir  func() (string, error)

	once        sync.Once
	initialErr  error
	credentials sync.Map
}

type authenticatedHTTPClient struct {
	client httpDoer
	auth   *GoAuthenticator
}

func (c authenticatedHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.auth.Do(c.client, req)
}

func (a *GoAuthenticator) Client(client httpDoer) httpDoer {
	if client == nil {
		client = http.DefaultClient
	}
	if a == nil {
		return client
	}
	return authenticatedHTTPClient{client: client, auth: a}
}

func (a *GoAuthenticator) GitAuth(ctx context.Context, repoURL string) transport.AuthMethod {
	parsed, err := url.Parse(repoURL)
	if err != nil || parsed.Scheme != "https" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, repoURL, nil)
	if err != nil || a.addCredentials(req, nil, "") != nil {
		return nil
	}
	if username, password, ok := req.BasicAuth(); ok {
		return &githttp.BasicAuth{Username: username, Password: password}
	}
	const bearerPrefix = "Bearer "
	if value := req.Header.Get("Authorization"); strings.HasPrefix(value, bearerPrefix) {
		return &githttp.TokenAuth{Token: strings.TrimPrefix(value, bearerPrefix)}
	}
	return nil
}

func (a *GoAuthenticator) Do(client httpDoer, req *http.Request) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}
	request := req.Clone(req.Context())
	request.Header = req.Header.Clone()
	if request.URL.Scheme == "https" {
		if err := a.addCredentials(request, nil, ""); err != nil {
			return nil, err
		}
	}
	response, err := client.Do(request)
	if err != nil || request.URL.Scheme != "https" || response.StatusCode < 400 || response.StatusCode >= 500 {
		return response, err
	}
	response.Body.Close()

	retry := req.Clone(req.Context())
	retry.Header = req.Header.Clone()
	if err := a.addCredentials(retry, response, request.URL.String()); err != nil {
		return nil, err
	}
	return client.Do(retry)
}

func (a *GoAuthenticator) addCredentials(req *http.Request, response *http.Response, requestURL string) error {
	if a == nil || a.Config == "off" {
		return nil
	}
	a.once.Do(func() {
		a.initialErr = a.run(req.Context(), nil, "")
	})
	if a.initialErr != nil {
		return a.initialErr
	}
	if requestURL != "" {
		if err := a.run(req.Context(), response, requestURL); err != nil {
			return err
		}
	}
	a.loadCredential(req)
	return nil
}

func (a *GoAuthenticator) run(ctx context.Context, response *http.Response, requestURL string) error {
	commands := strings.Split(a.Config, ";")
	slices.Reverse(commands)
	for _, raw := range commands {
		args, err := splitQuotedFields(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("parsing GOAUTH command: %w", err)
		}
		if len(args) == 0 {
			return fmt.Errorf("GOAUTH encountered an empty command (GOAUTH=%s)", a.Config)
		}
		switch args[0] {
		case "off":
			if len(commands) != 1 {
				return fmt.Errorf("GOAUTH=off cannot be combined with other authentication commands (GOAUTH=%s)", a.Config)
			}
			return nil
		case "netrc":
			if len(args) != 1 {
				return errors.New("GOAUTH=netrc does not accept arguments")
			}
			lines, err := a.readNetrc()
			if err != nil {
				continue
			}
			for i := len(lines) - 1; i >= 0; i-- {
				header := make(http.Header)
				req := http.Request{Header: header}
				req.SetBasicAuth(lines[i].Login, lines[i].Password)
				a.storeCredential(lines[i].Machine, header)
			}
		case "git":
			if len(args) != 2 || !filepath.IsAbs(args[1]) {
				return errors.New("GOAUTH=git requires one absolute working directory")
			}
			if err := validateGitCredentialDir(args[1]); err != nil {
				return err
			}
			if requestURL == "" {
				continue
			}
			prefix, header, err := runGitCredential(ctx, args[1], requestURL)
			if err != nil {
				continue
			}
			a.storeCredential(prefix, header)
		default:
			output, err := a.runner().Run(ctx, args, requestURL, response)
			if err != nil {
				continue
			}
			credentials, err := parseGoAuthOutput(string(output))
			if err != nil {
				continue
			}
			for prefix, header := range credentials {
				a.storeCredential(prefix, header)
			}
		}
	}
	return nil
}

func (a *GoAuthenticator) loadCredential(req *http.Request) bool {
	prefix := strings.TrimSuffix(strings.TrimPrefix(req.URL.String(), "https://"), "/")
	for {
		if stored, ok := a.credentials.Load(prefix); ok {
			for name, values := range stored.(http.Header) {
				for _, value := range values {
					req.Header.Add(name, value)
				}
			}
			return true
		}
		i := strings.LastIndexByte(prefix, '/')
		if i < 0 {
			return false
		}
		prefix = prefix[:i]
	}
}

func (a *GoAuthenticator) storeCredential(prefix string, header http.Header) {
	prefix = strings.TrimSuffix(strings.TrimPrefix(prefix, "https://"), "/")
	if len(header) == 0 {
		a.credentials.Delete(prefix)
		return
	}
	a.credentials.Store(prefix, header.Clone())
}

func (a *GoAuthenticator) runner() goAuthCommandRunner {
	if a.Runner != nil {
		return a.Runner
	}
	return execGoAuthCommandRunner{}
}

type netrcCredential struct {
	Machine  string
	Login    string
	Password string
}

func (a *GoAuthenticator) readNetrc() ([]netrcCredential, error) {
	name, err := a.netrcPath()
	if err != nil {
		return nil, err
	}
	readFile := a.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	data, err := readFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseNetrc(string(data)), nil
}

func (a *GoAuthenticator) netrcPath() (string, error) {
	lookup := a.Lookup
	if lookup == nil {
		lookup = os.LookupEnv
	}
	if name, ok := lookup("NETRC"); ok && name != "" {
		return name, nil
	}
	homeDir := a.HomeDir
	if homeDir == nil {
		homeDir = os.UserHomeDir
	}
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		legacy := filepath.Join(home, "_netrc")
		if _, err := os.Stat(legacy); err == nil {
			return legacy, nil
		}
	}
	return filepath.Join(home, ".netrc"), nil
}

func parseNetrc(data string) []netrcCredential {
	var credentials []netrcCredential
	var current netrcCredential
	inMacro := false
	for line := range strings.SplitSeq(data, "\n") {
		if inMacro {
			if line == "" {
				inMacro = false
			}
			continue
		}
		fields := strings.Fields(line)
		i := 0
		for ; i < len(fields)-1; i += 2 {
			switch fields[i] {
			case "machine":
				current = netrcCredential{Machine: fields[i+1]}
			case "default":
				break
			case "login":
				current.Login = fields[i+1]
			case "password":
				current.Password = fields[i+1]
			case "macdef":
				inMacro = true
			}
			if current.Machine != "" && current.Login != "" && current.Password != "" {
				credentials = append(credentials, current)
				current = netrcCredential{}
			}
		}
		if i < len(fields) && fields[i] == "default" {
			break
		}
	}
	return credentials
}

func parseGoAuthOutput(data string) (map[string]http.Header, error) {
	credentials := make(map[string]http.Header)
	for data != "" {
		var prefixes []string
		for {
			line, rest, ok := strings.Cut(data, "\n")
			if !ok {
				return nil, errors.New("invalid format: missing empty line after URLs")
			}
			data = rest
			if line == "" {
				break
			}
			parsed, err := url.ParseRequestURI(line)
			if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
				return nil, fmt.Errorf("invalid credential URL %q", line)
			}
			prefixes = append(prefixes, parsed.String())
		}

		header := make(http.Header)
		for {
			line, rest, ok := strings.Cut(data, "\n")
			if !ok {
				return nil, errors.New("invalid format: missing empty line after headers")
			}
			data = rest
			if line == "" {
				break
			}
			name, value, ok := strings.Cut(line, ":")
			value = strings.TrimSpace(value)
			if !ok || !validHTTPHeaderName(name) || !validHTTPHeaderValue(value) {
				return nil, errors.New("invalid format: invalid header line")
			}
			header.Add(name, value)
		}
		for _, prefix := range prefixes {
			credentials[prefix] = header.Clone()
		}
	}
	return credentials, nil
}

func runGitCredential(ctx context.Context, dir, requestURL string) (string, http.Header, error) {
	if err := validateGitCredentialDir(dir); err != nil {
		return "", nil, err
	}
	cmd := exec.CommandContext(ctx, "git", "credential", "fill")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader("url=" + requestURL + "\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", nil, fmt.Errorf("git credential fill: %w: %s", err, strings.TrimSpace(string(output)))
	}
	prefix, username, password := parseGitCredential(output)
	if prefix == "" || !strings.HasPrefix(requestURL, prefix) {
		return "", nil, fmt.Errorf("git credential returned non-matching URL %q", prefix)
	}
	header := make(http.Header)
	req := http.Request{Header: header}
	req.SetBasicAuth(username, password)
	return prefix, header, nil
}

func validateGitCredentialDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("checking GOAUTH git directory: %w", err)
	}
	if !info.IsDir() {
		return errors.New("GOAUTH git path is not a directory")
	}
	return nil
}

func parseGitCredential(data []byte) (prefix, username, password string) {
	parsed := new(url.URL)
	for line := range strings.SplitSeq(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "protocol":
			parsed.Scheme = value
		case "host":
			parsed.Host = value
		case "path":
			parsed.Path = value
		case "username":
			username = value
		case "password":
			password = value
		case "url":
			if valueURL, err := url.ParseRequestURI(value); err == nil {
				parsed = valueURL
			}
		}
	}
	return parsed.String(), username, password
}

func formatAuthResponse(response *http.Response) string {
	var output strings.Builder
	output.WriteString(response.Proto)
	output.WriteByte(' ')
	output.WriteString(response.Status)
	output.WriteByte('\n')
	for name, values := range response.Header {
		output.WriteString(name)
		output.WriteString(": ")
		output.WriteString(strings.Join(values, ", "))
		output.WriteByte('\n')
	}
	output.WriteByte('\n')
	return output.String()
}

func splitQuotedFields(value string) ([]string, error) {
	var fields []string
	for len(value) > 0 {
		value = strings.TrimLeft(value, " \t\n\r")
		if value == "" {
			break
		}
		if value[0] == '\'' || value[0] == '"' {
			quote := value[0]
			value = value[1:]
			i := strings.IndexByte(value, quote)
			if i < 0 {
				return nil, fmt.Errorf("unterminated %c string", quote)
			}
			fields = append(fields, value[:i])
			value = value[i+1:]
			continue
		}
		i := strings.IndexAny(value, " \t\n\r")
		if i < 0 {
			fields = append(fields, value)
			break
		}
		fields = append(fields, value[:i])
		value = value[i:]
	}
	return fields, nil
}

func validHTTPHeaderName(value string) bool {
	if value == "" {
		return false
	}
	for i := range len(value) {
		char := value[i]
		if !strings.ContainsRune("!#$%&'*+-.^_`|~", rune(char)) &&
			(char < '0' || char > '9') &&
			(char < 'A' || char > 'Z') &&
			(char < 'a' || char > 'z') {
			return false
		}
	}
	return true
}

func validHTTPHeaderValue(value string) bool {
	for i := range len(value) {
		if value[i] == '\r' || value[i] == '\n' || value[i] == 0x7f || value[i] < ' ' && value[i] != '\t' {
			return false
		}
	}
	return true
}
