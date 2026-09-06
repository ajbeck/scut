---

title: "scut gotools"
description: "Go documentation lookup commands for agents."
kicker: "CLI Reference"
tags: ["Go", "docs"]
weight: 90
---
`scut gotools` provides Go tool-inspired lookups that are formatted for agent consumption.

## Source resolution

`scut gotools doc` first checks the current package and standard library before
trying its external source routes.

For an external package in an active Go module or workspace, it then consults
the Go-selected build list. Workspace modules and local replacements are read
from their source directories. External build-list directories beneath the Go
module cache are used for version selection only; they are never treated as
package source. Build-list discovery is best-effort: if it cannot run, the
command uses direct requirements from the active `go.mod` and the remaining
local and remote sources.

It can resolve an arbitrary external package through the configured Go module
proxy or direct Git policy, so the package does not need to be in the current
project's `go.mod`.

Scut's documentation source fetchers never write fetched package files into
`GOMODCACHE`. An existing Go download-cache entry is reused only when its
canonical module ZIP and `.ziphash` are both present, the ZIP is structurally
valid, and its computed content hash matches `.ziphash`. The Go command writes
that sidecar only after applying its checksum policy. Scut also rejects the
entry if it conflicts with an applicable checksum in the active module or
workspace. Extracted module
directories are not read as source or indexed for shorthand package discovery.
Missing or invalid cache artifacts are not repaired or extracted in place.

Build-list discovery currently invokes `go list -mod=readonly -m -json all`.
That command does not edit the active `go.mod`, but the Go command may perform
its own normal module-cache work while loading the build list.

### Active build context

After a source backend resolves the complete package source, scut applies one
shared Go build context before parsing documentation. This keeps cached module
archives complete and target-independent while making local, standard-library,
Go-cache, scut-cache, proxy, and direct-Git results select the same files.

The target uses `GOOS`, `GOARCH`, and `CGO_ENABLED` from the process environment,
the user `go/env` file, or `GOROOT/go.env` with Go's normal precedence.
Configured `-tags` or `--tags` values in `GOFLAGS` are added to the context;
the running scut process supplies compiler, tool, and release tags from its Go
toolchain. Scut does not copy the Go command's private machinery for
recomputing experimental or microarchitecture tool tags from a different target
stored only in a Go environment file. Scut honors both `//go:build` expressions
and GOOS/GOARCH filename suffixes. Files that import `C` are excluded when cgo
is disabled, and `_test.go`, dot-prefixed, and underscore-prefixed files are not
included in package documentation.

If a resolved package has source but the active context excludes every file,
the command reports that build constraints exclude all Go files instead of
misreporting the package as absent.

### Go network policy

Scut reads module download settings without invoking the Go command. Values use
the same precedence as Go: a non-empty process environment value, the user
`go/env` file unless `GOENV=off`, and then `GOROOT/go.env`. This includes
settings written by `go env -w` and the toolchain default
`GOPROXY=https://proxy.golang.org,direct`.

Remote lookup is one ordered `GOPROXY` state machine:

- A comma advances only after a module-not-found result, including HTTP 404 or
  410. A pipe advances after any proxy error.
- `direct` performs repository discovery and an in-memory Git clone. `off`
  returns an actionable disabled error without attempting a remote source.
- HTTP, HTTPS, and `file://` module proxies are supported. Proxy host names
  without a scheme receive Go's implicit `https://` prefix.
- `GONOPROXY` defaults to `GOPRIVATE` when unset. A matching module takes
  the implicit direct route and is not sent to a configured proxy.

`GOAUTH` command execution and 4xx retry apply to HTTPS go-import discovery
and module proxy requests. Scut supports the Go `off`, `netrc`,
`git <absolute-dir>`, and custom-command forms plus prefix-scoped response
headers. A failing individual helper does not prevent later helpers or an
anonymous request from succeeding. Basic or Bearer credentials available from
the initial GOAUTH pass may also authenticate an HTTPS Git clone.

`GOINSECURE` applies only to matching modules fetched directly: it may permit
HTTP discovery or repository transport and relaxed HTTPS certificate
verification. It does not weaken an explicitly configured proxy or change
`GONOSUMDB`; archive verification consumes that independently resolved checksum
policy. Direct Git is also subject to `GOVCS`; scut returns an error
before cloning when the first matching rule disallows Git.

### Archive integrity

Every remotely fetched canonical module archive is structurally validated and
hashed before package files are extracted. Scut first checks applicable
`go.sum`, `go.work.sum`, and workspace-module sum files. A recorded `h1:`
mismatch is terminal and is never replaced by a checksum-database result.

When no sum is recorded, public modules are authenticated with the checksum
database selected by `GOSUMDB`. The default is `sum.golang.org`; an explicit
database URL and checksum-database proxying through `GOPROXY` are supported.
`GOSUMDB=off` and matching `GONOSUMDB` patterns skip the public database but do
not skip structural validation or content hashing.

The checksum client keeps its latest signed tree checkpoint beneath scut's user
configuration directory, separate from both `GOMODCACHE` and the disposable
module archive cache. This preserves cross-process consistency and rollback
checks. Authenticated lookup records and tiles stay in memory because the
verified module archive itself becomes the reusable cache entry.

An unversioned public direct-Git checkout cannot be addressed in a checksum
database. Scut rejects that case with guidance to specify an exact `@version`;
floating direct lookups remain available when checksum policy explicitly
exempts the module.

### Independent module cache

The command persists complete immutable module ZIPs in a scut-owned cache below
the operating system's user cache directory at `scut/gotools/modules`. Entries
are structurally validated and published atomically, so concurrent lookups
cannot observe a partially written module. A failed cache write does not discard
documentation source that was fetched successfully for the current invocation.

For a project-selected dependency, the selected exact version is checked first
in the verified Go download cache and then in the scut cache. A proxy miss then
requests that same version rather than resolving `latest`. Versioned module
replacements retain the original import identity while reading the replacement
module's archive.

For a proxy `latest` request, scut records the concrete version returned by the
proxy and reuses that mapping for an offline lookup. Explicit canonical versions
fetched from private Git repositories are cached only when the clone exposes
the resolved commit. Floating private Git requests remain in memory.

Newly published entries include a self-generated `h1:` sidecar and immutable
verification provenance bound to that hash. The provenance records whether the
archive was accepted by an applicable sum, a checksum database, the Go cache,
or an explicit checksum-policy exemption. An entry is readable only when its
ZIP, self-hash, and provenance agree. Missing legacy metadata is reported as
incomplete so users can remove and refetch the entry explicitly.

### Cache management

`scut gotools cache` operates only on the scut-owned module archive cache. It
never removes or repairs anything in `GOMODCACHE`, and it does not erase the
separate signed checksum-database checkpoint.

- `cache path` prints the absolute owned-cache path.
- `cache list [MODULE[@VERSION]]` reports canonical entries, byte sizes,
  publication times, verification provenance, `latest` aliases, and validation
  state.
- `cache verify [MODULE[@VERSION]]` validates archive structure, the stored
  self-hash, and verification provenance without repairing anything. It returns
  a non-zero status when it finds an incomplete, malformed, or corrupted entry.
- `cache remove MODULE[@VERSION]` removes one exact version. Omitting the
  version removes every cached version of that module while preserving nested
  module paths.
- `cache clean` idempotently removes the complete scut-owned module cache.
- `cache prune --older-than=DURATION` removes entries at or older than the
  publication cutoff. `cache prune --max-size=SIZE` removes oldest entries until
  the cache fits. When both are supplied, age pruning runs first. Durations use
  Go duration syntax such as `720h`; sizes accept bytes and SI or IEC units such
  as `500MB` and `1.5GiB`. Pruning removes only canonical version entries. If
  unassociated malformed artifacts prevent the requested maximum size, it
  reports an error and directs the user to `cache verify` or `cache clean`.

`remove`, `clean`, and `prune` are intentionally non-interactive for agent and
automation use. Removing the version named by a module's `latest` alias removes
the alias rather than guessing another version. `list`, `verify`, and mutation
commands support `--json`; JSON verification output is written before the
command returns a failing status.

For one-argument lookups such as `example.com/module/pkg.Type`, the command
preserves `go doc`'s interpretation order: it first considers the full package
path, then package-and-symbol interpretations. A fetch failure for one
ambiguous interpretation does not prevent the others from being considered.

When the selected cached module version contains the requested package
directory but no Go source files, the lookup reports `package <path> not
found` without attempting remote fallback. A missing cache directory is not
conclusive and still permits remote resolution.

### Private GitHub repositories

For a private GitHub repository on a direct route, Git resolution tries HTTPS
first. It uses the first available token from `GH_TOKEN`, `GITHUB_TOKEN`, and
`GIT_TOKEN`. Compatible Basic or Bearer credentials already supplied by
`GOAUTH` are considered next. If none is available, scut makes a one-second
best-effort call to `gh auth token --hostname <host>`. A missing, failing, or
unauthenticated `gh` command is ignored.

When the active build list selects an exact direct module version, scut maps it
to Go's repository layout instead of assuming a root tag. Nested modules use
tags such as `subdir/v1.2.3`, semantic-import-version modules locate their
`v2/` source directory, and pseudo-versions select their encoded revision.
Versioned replacements preserve the logical import path while cloning and
caching the replacement module identity.

If an authenticated HTTPS clone of a `github.com` repository fails with a Git
authentication or authorization error, the command retries once through the
local SSH agent. It does not retry connection, TLS, DNS, or repository lookup
errors, and it keeps the original HTTPS authentication error if SSH is not
available or the retry fails.

## Generated help

{{< clihelp file="scut-gotools" command="scut gotools --help" >}}

{{< clihelp file="scut-gotools-doc" command="scut gotools doc --help" >}}

{{< clihelp file="scut-gotools-cache" command="scut gotools cache --help" >}}

{{< clihelp file="scut-gotools-cache-path" command="scut gotools cache path --help" >}}

{{< clihelp file="scut-gotools-cache-list" command="scut gotools cache list --help" >}}

{{< clihelp file="scut-gotools-cache-verify" command="scut gotools cache verify --help" >}}

{{< clihelp file="scut-gotools-cache-remove" command="scut gotools cache remove --help" >}}

{{< clihelp file="scut-gotools-cache-clean" command="scut gotools cache clean --help" >}}

{{< clihelp file="scut-gotools-cache-prune" command="scut gotools cache prune --help" >}}
