package godoc

import (
	"context"
	"errors"
)

// SourceFetcher loads package source for one route.
type SourceFetcher interface {
	Fetch(context.Context, string, Options) (PackageSource, error)
}

// Resolver tries source fetchers in configured priority order.
type Resolver struct {
	Fetchers []SourceFetcher
}

func (r Resolver) Fetch(ctx context.Context, pkg string, opts Options) (PackageSource, error) {
	for _, fetcher := range r.Fetchers {
		source, err := fetcher.Fetch(ctx, pkg, opts)
		if err == nil {
			return source, nil
		}
		if errors.Is(err, ErrSourceNotApplicable) {
			continue
		}
		return PackageSource{}, err
	}
	return PackageSource{}, PackageNotFoundError{Package: pkg}
}
