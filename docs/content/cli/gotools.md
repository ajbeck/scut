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
the Go-selected build list and reads the selected source directory, including
local replacements, before checking the module cache. Build-list discovery is
best-effort: if it cannot run, the command uses the remaining local and remote
sources as usual.

It can resolve an arbitrary external package from a private Git repository or
public module proxy, so the package does not need to be in the current
project's `go.mod`.

Remote documentation lookup does not write fetched package files into
`GOMODCACHE`. The Go module cache is treated as an existing source only. This
prevents a documentation lookup from creating a partial extracted module that
could break later Go commands.

### Independent module cache

The command persists complete immutable module ZIPs in a scut-owned cache below
the operating system's user cache directory at `scut/gotools/modules`. Entries
are structurally validated and published atomically, so concurrent lookups
cannot observe a partially written module. A failed cache write does not discard
documentation source that was fetched successfully for the current invocation.

For a proxy `latest` request, scut records the concrete version returned by the
proxy and reuses that mapping for an offline lookup. Explicit canonical versions
fetched from private Git repositories are cached only when the clone exposes
the resolved commit. Floating private Git requests remain in memory.

The cache has no automatic eviction. Explicit inspection and lifecycle commands
will be added separately.

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
