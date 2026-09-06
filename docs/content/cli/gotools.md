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

It can resolve an arbitrary external package from a private Git repository or
public module proxy, so the package does not need to be in the current
project's `go.mod`.

Scut's documentation source fetchers never write fetched package files into
`GOMODCACHE`. An existing Go download-cache entry is reused only when its
canonical module ZIP and `.ziphash` are both present, the ZIP is structurally
valid, and its computed content hash matches `.ziphash`. Extracted module
directories are not read as source or indexed for shorthand package discovery.
Missing or invalid cache artifacts are not repaired or extracted in place.

Build-list discovery currently invokes `go list -mod=readonly -m -json all`.
That command does not edit the active `go.mod`, but the Go command may perform
its own normal module-cache work while loading the build list.

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

Newly published entries include a self-generated `h1:` sidecar. This detects
storage corruption but is not a source-authenticity decision; verification
against `go.sum` or a checksum database is a separate integrity layer. Entries
created before the sidecar was introduced remain readable but `cache verify`
reports them as incomplete so users can remove and refetch them explicitly.

### Cache management

`scut gotools cache` operates only on the scut-owned module archive cache. It
never removes or repairs anything in `GOMODCACHE`.

- `cache path` prints the absolute owned-cache path.
- `cache list [MODULE[@VERSION]]` reports canonical entries, byte sizes,
  publication times, `latest` aliases, and validation state.
- `cache verify [MODULE[@VERSION]]` validates archive structure and the stored
  self-hash without repairing anything. It returns a non-zero status when it
  finds an incomplete, malformed, or corrupted entry.
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

For a private GitHub repository, direct Git resolution tries HTTPS first. It
uses the first available token from `GH_TOKEN`, `GITHUB_TOKEN`, and
`GIT_TOKEN`; if none is set, it makes a one-second best-effort call to `gh auth
token --hostname <host>`. A missing, failing, or unauthenticated `gh` command
is ignored, so the lookup can continue through the usual source routes.

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
