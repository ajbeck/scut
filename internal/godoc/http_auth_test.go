package godoc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeGoAuthRunner struct {
	calls     []string
	responses []string
	output    func(string) []byte
	err       error
}

func (r *fakeGoAuthRunner) Run(_ context.Context, _ []string, requestURL string, response *http.Response) ([]byte, error) {
	r.calls = append(r.calls, requestURL)
	if response != nil {
		r.responses = append(r.responses, formatAuthResponse(response))
	}
	if r.err != nil {
		return nil, r.err
	}
	if r.output != nil {
		return r.output(requestURL), nil
	}
	return nil, nil
}

type httpDoerFunc func(*http.Request) (*http.Response, error)

func (f httpDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGoAuthenticatorLoadsNetrcByURLPrefix(t *testing.T) {
	auth := &GoAuthenticator{
		Config:   "netrc",
		HomeDir:  func() (string, error) { return "/home/test", nil },
		ReadFile: func(string) ([]byte, error) { return []byte("machine proxy.example login user password secret\n"), nil },
		Lookup:   func(string) (string, bool) { return "", false },
	}
	client := httpDoerFunc(func(req *http.Request) (*http.Response, error) {
		username, password, ok := req.BasicAuth()
		if !ok || username != "user" || password != "secret" {
			t.Fatalf("BasicAuth() = %q, %q, %v", username, password, ok)
		}
		return authResponse(http.StatusOK), nil
	})
	req, _ := http.NewRequest(http.MethodGet, "https://proxy.example/module/@latest", nil)
	if _, err := auth.Do(client, req); err != nil {
		t.Fatalf("Do() error = %v", err)
	}
}

func TestGoAuthenticatorRetriesOnceAfter4xx(t *testing.T) {
	runner := &fakeGoAuthRunner{output: func(requestURL string) []byte {
		if requestURL == "" {
			return nil
		}
		return []byte("https://proxy.example/\n\nAuthorization: Bearer retry-secret\n\n")
	}}
	auth := &GoAuthenticator{Config: "credential-helper", Runner: runner}
	requests := 0
	client := httpDoerFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			if got := req.Header.Get("Authorization"); got != "" {
				t.Fatalf("first Authorization = %q", got)
			}
			return authResponse(http.StatusUnauthorized), nil
		}
		if got, want := req.Header.Get("Authorization"), "Bearer retry-secret"; got != want {
			t.Fatalf("retry Authorization = %q, want %q", got, want)
		}
		return authResponse(http.StatusOK), nil
	})
	req, _ := http.NewRequest(http.MethodGet, "https://proxy.example/module/@latest", nil)
	response, err := auth.Do(client, req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if response.StatusCode != http.StatusOK || requests != 2 {
		t.Fatalf("response = %d after %d requests", response.StatusCode, requests)
	}
	if len(runner.calls) != 2 || runner.calls[0] != "" || runner.calls[1] != req.URL.String() {
		t.Fatalf("runner calls = %#v", runner.calls)
	}
	if len(runner.responses) != 1 || !strings.Contains(runner.responses[0], "401 Unauthorized") {
		t.Fatalf("runner responses = %#v", runner.responses)
	}
}

func TestGoAuthenticatorDoesNotSendCredentialsOverHTTP(t *testing.T) {
	runner := &fakeGoAuthRunner{err: errors.New("must not run")}
	auth := &GoAuthenticator{Config: "credential-helper", Runner: runner}
	client := httpDoerFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization = %q", got)
		}
		return authResponse(http.StatusOK), nil
	})
	req, _ := http.NewRequest(http.MethodGet, "http://proxy.example/module/@latest", nil)
	if _, err := auth.Do(client, req); err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner calls = %#v", runner.calls)
	}
}

func TestGoAuthenticatorRejectsInvalidConfiguration(t *testing.T) {
	auth := &GoAuthenticator{Config: "off;netrc"}
	client := httpDoerFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("HTTP request should not run")
		return nil, nil
	})
	req, _ := http.NewRequest(http.MethodGet, "https://proxy.example/module/@latest", nil)
	if _, err := auth.Do(client, req); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("Do() error = %v", err)
	}
}

func TestGoAuthenticatorContinuesAfterHelperFailure(t *testing.T) {
	runner := &fakeGoAuthRunner{err: errors.New("helper unavailable")}
	auth := &GoAuthenticator{
		Config:   "helper;netrc",
		Runner:   runner,
		HomeDir:  func() (string, error) { return "/home/test", nil },
		ReadFile: func(string) ([]byte, error) { return []byte("machine proxy.example login user password secret\n"), nil },
		Lookup:   func(string) (string, bool) { return "", false },
	}
	client := httpDoerFunc(func(req *http.Request) (*http.Response, error) {
		if username, password, ok := req.BasicAuth(); !ok || username != "user" || password != "secret" {
			t.Fatalf("BasicAuth() = %q, %q, %v", username, password, ok)
		}
		return authResponse(http.StatusOK), nil
	})
	req, _ := http.NewRequest(http.MethodGet, "https://proxy.example/module", nil)
	if _, err := auth.Do(client, req); err != nil {
		t.Fatalf("Do() error = %v", err)
	}
}

func TestParseGoAuthOutputRejectsUnsafeHeader(t *testing.T) {
	_, err := parseGoAuthOutput("https://proxy.example/\n\nAuthorization: bad\rvalue\n\n")
	if err == nil {
		t.Fatal("parseGoAuthOutput() error = nil")
	}
}

func authResponse(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Proto:      "HTTP/1.1",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
	}
}
