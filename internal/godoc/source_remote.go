package godoc

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/mod/module"
)

type proxyRouteKind uint8

const (
	proxyRouteURL proxyRouteKind = iota
	proxyRouteNoProxy
	proxyRouteDirect
	proxyRouteOff
)

type proxyRoute struct {
	Kind            proxyRouteKind
	URL             string
	FallbackOnError bool
}

var (
	errProxyOff error = remoteNotFoundError("module lookup disabled by GOPROXY=off")
	errNoProxy  error = remoteNotFoundError("disabled by GOPRIVATE/GONOPROXY")
	errUseProxy error = remoteNotFoundError("path does not match GOPRIVATE/GONOPROXY")
)

type remoteNotFoundError string

func (e remoteNotFoundError) Error() string {
	return string(e)
}

func (remoteNotFoundError) Is(target error) bool {
	return target == fs.ErrNotExist
}

func parseProxyRoutes(policy ModuleDownloadPolicy) ([]proxyRoute, error) {
	var routes []proxyRoute
	if policy.GONOPROXY != "" && policy.GOPROXY != "direct" {
		routes = append(routes, proxyRoute{Kind: proxyRouteNoProxy})
	}

	remaining := policy.GOPROXY
	for remaining != "" {
		entry := remaining
		fallbackOnError := false
		if i := strings.IndexAny(remaining, ",|"); i >= 0 {
			entry = remaining[:i]
			fallbackOnError = remaining[i] == '|'
			remaining = remaining[i+1:]
		} else {
			remaining = ""
		}
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		switch entry {
		case "off":
			routes = append(routes, proxyRoute{Kind: proxyRouteOff})
			remaining = ""
			continue
		case "direct":
			routes = append(routes, proxyRoute{Kind: proxyRouteDirect})
			remaining = ""
			continue
		}

		normalized, err := normalizeProxyURL(entry)
		if err != nil {
			return nil, err
		}
		routes = append(routes, proxyRoute{
			Kind:            proxyRouteURL,
			URL:             normalized,
			FallbackOnError: fallbackOnError,
		})
	}
	if len(routes) == 0 || len(routes) == 1 && routes[0].Kind == proxyRouteNoProxy {
		return nil, errors.New("GOPROXY list is not the empty string, but contains no entries")
	}
	return routes, nil
}

func normalizeProxyURL(value string) (string, error) {
	if strings.ContainsAny(value, ".:/") && !strings.Contains(value, ":/") && !filepath.IsAbs(value) && !path.IsAbs(value) {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "http", "https":
	case "file":
		if *parsed != (url.URL{Scheme: parsed.Scheme, Path: parsed.Path, RawPath: parsed.RawPath}) {
			return "", fmt.Errorf("invalid file:// proxy URL with non-path elements: %s", parsed.Redacted())
		}
	case "":
		return "", fmt.Errorf("invalid proxy URL missing scheme: %s", parsed.Redacted())
	default:
		return "", fmt.Errorf("invalid proxy URL scheme (must be https, http, file): %s", parsed.Redacted())
	}
	return value, nil
}

func matchesModulePattern(patterns, modulePath string) bool {
	return patterns != "" && module.MatchPrefixPatterns(patterns, modulePath)
}

type remoteRouteFetchFunc func(context.Context, proxyRoute, string, Options) (PackageSource, error)

// RemoteFetcher applies Go's proxy fallback policy around proxy and direct
// source mechanisms.
type RemoteFetcher struct {
	Policy ModuleDownloadPolicy
	Proxy  ProxyFetcher
	Direct GitFetcher

	fetchRoute remoteRouteFetchFunc
}

func (f RemoteFetcher) Fetch(ctx context.Context, pkg string, opts Options) (PackageSource, error) {
	routes, err := parseProxyRoutes(f.Policy)
	if err != nil {
		return PackageSource{}, err
	}

	const (
		notFoundRank = iota
		proxyRank
		directRank
	)
	var bestErr error
	bestRank := notFoundRank
	for _, route := range routes {
		source, err := f.routeFetcher()(ctx, route, pkg, opts)
		if err == nil {
			return source, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return PackageSource{}, ctxErr
		}
		notFound := isRemoteNotFound(err)
		switch {
		case route.Kind == proxyRouteDirect || route.Kind == proxyRouteNoProxy && !errors.Is(err, errUseProxy):
			bestErr, bestRank = err, directRank
		case bestRank <= proxyRank && !notFound:
			bestErr, bestRank = err, proxyRank
		case bestErr == nil:
			bestErr = err
		}
		if !route.FallbackOnError && !notFound {
			break
		}
	}
	if bestErr == nil {
		return PackageSource{}, ErrSourceNotApplicable
	}
	return PackageSource{}, bestErr
}

func (f RemoteFetcher) routeFetcher() remoteRouteFetchFunc {
	if f.fetchRoute != nil {
		return f.fetchRoute
	}
	return f.fetch
}

func (f RemoteFetcher) fetch(ctx context.Context, route proxyRoute, pkg string, opts Options) (PackageSource, error) {
	switch route.Kind {
	case proxyRouteNoProxy:
		direct := f.Direct
		direct.GOPRIVATE = f.Policy.GOPRIVATE
		direct.GONOPROXY = f.Policy.GONOPROXY
		direct.GOINSECURE = f.Policy.GOINSECURE
		direct.GOVCS = f.Policy.GOVCS
		direct.RequireNoProxy = true
		direct.AllowPublic = true
		return direct.Fetch(ctx, pkg, opts)
	case proxyRouteDirect:
		direct := f.Direct
		direct.GOPRIVATE = f.Policy.GOPRIVATE
		direct.GOINSECURE = f.Policy.GOINSECURE
		direct.GOVCS = f.Policy.GOVCS
		direct.RequireNoProxy = false
		direct.AllowPublic = true
		return direct.Fetch(ctx, pkg, opts)
	case proxyRouteOff:
		return PackageSource{}, errProxyOff
	case proxyRouteURL:
		proxy := f.Proxy
		proxy.ProxyURL = route.URL
		proxy.Exclude = func(modulePath string) bool {
			return matchesModulePattern(f.Policy.GONOPROXY, modulePath)
		}
		return proxy.Fetch(ctx, pkg, opts)
	default:
		return PackageSource{}, errors.New("unknown module proxy route")
	}
}

func isRemoteNotFound(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrSourceNotApplicable)
}

var _ SourceFetcher = RemoteFetcher{}
