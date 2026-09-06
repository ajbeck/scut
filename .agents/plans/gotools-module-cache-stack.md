# Go tools module cache stack

Status: active\
Last updated: 2026-09-06\
Repository: `ajbeck/scut`\
Trunk: `main`

## Objective

Make `scut gotools doc` resolve external Go documentation in-process without
creating or modifying a project `go.mod`, while never corrupting the Go module
cache. Scut will own a separate cache of complete immutable module archives and
will treat `GOMODCACHE` as a verified, read-only source.

This document is the durable implementation plan and decision log. Update it
when a layer changes status, a design decision is made, new work is discovered,
or the stack shape changes.

## Invariants

- Scut's in-process source fetchers and stores never write, extract, repair,
  chmod, or otherwise mutate `GOMODCACHE`; the retained Go build-list probe may
  perform normal Go-managed cache work.
- Never persist package fragments as though they were complete modules.
- Scut-owned persistent entries are complete immutable module archives keyed by
  a concrete module identity.
- Local modules, workspace modules, replacements, and the standard library take
  precedence over remote module sources.
- Existing verified Go-cache archives may be reused read-only before consulting
  the scut cache; the scut cache is the primary persistent store owned and
  written by scut.
- Remote retrieval remains in-process and does not require a temporary `go.mod`.
- Every stack layer must pass the Walle format, test, vet, and build tasks.
- Behavioral documentation changes ship in the same layer as the behavior.
- Do not push stack branches or open PRs without explicit user approval.
- Do not add a direct dependency without explicit user approval.

## Stack

The stack is linear and listed bottom-to-top.

| Layer | Branch                             | Issue                                           | Status                       | Scope                                                                                               |
| ----- | ---------------------------------- | ----------------------------------------------- | ---------------------------- | --------------------------------------------------------------------------------------------------- |
| 1     | `gotools-cache/safety`             | [#50](https://github.com/ajbeck/scut/issues/50) | Implemented; awaiting review | Stop all writes of partial modules into `GOMODCACHE`; add an isolated regression.                   |
| 2     | `gotools-cache/archive-store`      | [#51](https://github.com/ajbeck/scut/issues/51) | Implemented; awaiting review | Add the scut-owned complete immutable archive store with atomic publication and concurrency safety. |
| 3     | `gotools-cache/go-cache-reader`    | [#52](https://github.com/ajbeck/scut/issues/52) | Implemented; awaiting review | Reuse verified Go download-cache archives read-only and establish final source ordering.            |
| 4     | `gotools-cache/commands`           | [#53](https://github.com/ajbeck/scut/issues/53) | Implemented; awaiting review | Add path, list, verify, remove, clean, and prune cache-management commands.                         |
| 5     | `gotools-resolution/proxy-policy`  | [#54](https://github.com/ajbeck/scut/issues/54) | Implemented; awaiting review | Match Go proxy fallback, private-module, authentication, and transport policy.                      |
| 6     | `gotools-resolution/integrity`     | [#55](https://github.com/ajbeck/scut/issues/55) | Planned                      | Verify archive structure and checksums before consumption or publication.                           |
| 7     | `gotools-resolution/build-context` | [#56](https://github.com/ajbeck/scut/issues/56) | Planned                      | Honor build constraints and add full-pipeline integration coverage and final documentation.         |

## Layer 1 implementation plan

1. Map every construction and invocation of `WriteCache`, `CacheFS`, and
   `CacheDir` in the Go documentation source pipeline.
2. Add a regression around a fresh isolated `GOMODCACHE` that demonstrates the
   old partial-module write and protects the Go cache from any mutation.
3. Remove the proxy and private-Git cache writes and the now-dead fragment-cache
   writer without changing fetch precedence or successful source results.
4. Update `docs/content/cli/gotools.md` to document the safety boundary that
   `gotools doc` does not populate or modify `GOMODCACHE`.
5. Run `./walle fmt`, `./walle test`, `./walle vet`, and `./walle build`.
6. Review the branch diff against issue #50 and commit with conventional syntax.
7. Rebase all higher stack layers after any bottom-layer commit.

## Layer 2 implementation plan

1. Define a complete `ModuleArchive` and an `ArchiveStore` boundary for exact
   canonical module versions.
2. Implement a filesystem store beneath `os.UserCacheDir()` using an atomic
   directory-per-version publication protocol.
3. Validate module ZIP structure with the existing `golang.org/x/mod/zip`
   package before an entry becomes visible.
4. Persist and resolve an atomic `latest` alias only when a proxy `@latest`
   request returns a concrete version.
5. Add an archive source before remote sources so cached public and explicitly
   versioned private modules work without network access.
6. Teach proxy fetching to retain and cache the complete downloaded ZIP.
7. Teach private Git cloning to return its resolved commit and create a complete
   canonical module ZIP for explicit semantic versions.
8. Add store, concurrency, offline reuse, complete-content, and private Git
   regression tests.
9. Update gotools documentation and run all Walle verification tasks with Go
   1.26.3.

## Layer 3 implementation plan

1. Separate active build-list module selection from package source loading.
2. Let build-list loading serve only workspace modules and local replacements;
   never read an external module `Dir` rooted in extracted `GOMODCACHE`.
3. Represent the logical selected module separately from a versioned replacement
   module that physically supplies its archive.
4. Read canonical `.zip` and `.ziphash` files beneath
   `GOMODCACHE/cache/download` without creating, repairing, or extracting files.
5. Structurally validate the ZIP and compare its computed `h1:` content hash to
   the Go-owned `.ziphash` before exposing package source.
6. Share build-list selection with the Go-cache, scut-cache, and proxy sources so
   an absent local ZIP still fetches the project-selected version rather than
   proxy `latest`.
7. Remove `ModCacheFetcher` and its extracted-directory source tests.
8. Add read-only snapshots, incomplete-cache, hash-mismatch, build-list version,
   replacement, and source-order regression tests.
9. Update documentation and run all Walle verification tasks with Go 1.26.3.

## Layer 4 implementation plan

1. Add a cache inventory API that discovers only entries beneath the scut-owned
   root and reports canonical module identity, size, publication time, revision,
   latest-alias state, and validation problems deterministically.
2. Add an `h1:` self-hash sidecar to new scut archive entries and verify it when
   present, while recognizing pre-sidecar entries as incomplete legacy entries
   instead of making an unsafe migration assumption.
3. Add `scut gotools cache path`, `list`, and `verify` with stable human and JSON
   output; verification returns a non-zero error when any selected entry or
   cache artifact is invalid or incomplete.
4. Add narrowly targeted `remove <module[@version]>`; removing a module removes
   only its escaped cache subtree, while removing a version also clears a
   matching `latest` alias without retargeting it.
5. Add idempotent `clean` that removes only the resolved scut cache root.
6. Add `prune` with required `--older-than` and/or human-readable `--max-size`
   policies. Apply age first, then remove oldest remaining entries until the
   total owned-cache bytes are within the size bound.
7. Make destructive invocation sufficient intent, without interactive prompts,
   and report removed entry counts and bytes deterministically.
8. Add parser, inventory, validation, path-containment, alias, mutation, prune,
   JSON, and command-tree regressions.
9. Update gotools and architecture documentation and run all Walle verification
   tasks with Go 1.26.3.

## Layer 5 implementation plan

1. Load module-resolution settings with Go's precedence: non-empty process
   environment, the user `go/env` file unless `GOENV=off`, then
   `GOROOT/go.env`, without invoking the Go command.
2. Parse `GOPROXY` into an ordered state machine that preserves comma versus
   pipe fallback, terminal `direct` and `off` entries, implicit HTTPS proxy
   URLs, and the implicit `noproxy` route used by `GONOPROXY`.
3. Replace the independent Git and per-proxy fetcher chain with one remote
   fetcher that ranks errors and decides fallback once for the whole lookup.
4. Apply `GONOPROXY` to resolved module identities, default it from
   `GOPRIVATE`, and keep `GONOSUMDB` in the shared policy for layer 6.
5. Make direct Git resolution selector-aware, including versioned
   replacements, repository submodules, semantic-import-version suffixes,
   nested-module tag prefixes, canonical tags, and pseudo-version revisions.
6. Implement `GOAUTH` for HTTPS discovery and module-proxy requests, including
   `off`, `netrc`, `git <absolute-dir>`, custom commands, prefix-scoped headers,
   and the one retry after a 4xx response.
7. Apply `GOINSECURE` only to matching direct module discovery and Git
   transport, and enforce `GOVCS` before any direct Git clone.
8. Preserve the existing environment/GitHub CLI token preference and GitHub
   SSH retry where direct policy permits the clone.
9. Add parser, environment precedence, fallback, private override,
   authentication, insecure transport, VCS policy, selected-version, and
   nested-module tests.
10. Update gotools and architecture documentation and run all Walle
    verification tasks with Go 1.26.3.

## Decisions

### D-001: Independent cache ownership

The scut cache is the primary persistent store that scut owns and writes. A
verified archive already present in the Go download cache is nevertheless the
first external cache read because it avoids duplicate storage and network work.

### D-002: Immediate safety layer

The bottom PR removes unsafe `GOMODCACHE` writes without waiting for the new
cache. Until layer 2, a newly fetched external package is served from memory and
must be fetched again on a later invocation if no safe existing source contains
it.

### D-003: Complete immutable entries

The independent cache stores whole module archives, not selected package files.
Proxy downloads already provide the complete ZIP. Private Git data is persisted
only after a requested ref has been resolved to an immutable commit identity;
floating requests remain in-memory until that identity is known.

### D-004: Explicit lifecycle management

There is no automatic eviction initially. Users and agents manage the owned
cache through explicit commands introduced in layer 4.

### D-005: Tests stay with their behavior

Each layer carries its own unit and regression tests. Layer 7 adds cross-source
end-to-end coverage but is not a substitute for tests in earlier layers.

### D-006: Verification uses the declared Go toolchain

The host default Go 1.27 toolchain exposes a different experimental JSON-v2 API
surface and rejects existing repository code declared at Go 1.26.3. Walle tasks
for this stack are therefore invoked with `GOTOOLCHAIN=go1.26.3`, matching
`go.mod`, rather than changing unrelated JSON code or dependencies.

### D-007: Cached latest is an explicit alias

When a proxy `@latest` request resolves to a concrete version, the store records
that version as a small atomic alias only after the archive has been published.
Offline default lookups use this alias. They do not guess that the highest
cached semantic version is equivalent to Go proxy `@latest` behavior.

### D-008: Cache persistence is best-effort for document availability

A failed validation or filesystem operation never publishes a partial cache
entry, but it does not discard package source already fetched successfully for
the current documentation request. Cache-management commands will provide the
explicit diagnostics and repair surface in layer 4.

### D-009: One directory is one atomic cache entry

Each module version is staged as a sibling temporary directory containing the
complete canonical ZIP and optional immutable source revision. Renaming that
directory makes the whole entry visible at once. Concurrent writers either
publish a complete entry or accept an already-valid winner.

### D-010: Cache dependencies are injected below environment discovery

`NewDefaultClient` resolves the operating-system cache path, then delegates to
an internal client assembler that accepts an `ArchiveStore`. Tests use isolated
stores and cannot read or write the developer's real scut cache.

### D-011: Structural validation precedes checksum verification

Layer 2 uses `golang.org/x/mod/zip.CheckZip` to enforce canonical module ZIP
paths, versions, collisions, and size limits before publication and on reads.
Cryptographic verification against `go.sum` or the checksum database remains
owned by layer 6; structural validation is not presented as checksum proof.

### D-012: Build-list selection and source bytes are separate

The build list remains the authoritative selector for active modules and
replacements, but its external `Dir` values are not source roots. Only workspace
modules and local replacements may be read by directory. External bytes come
from verified Go-cache archives, scut-owned archives, or remote sources.

### D-013: Go-cache ziphash is required and read-only

A Go-cache archive is reusable only when both its canonical `.zip` and
`.ziphash` exist, the ZIP is structurally valid, and `dirhash.HashZip` matches
the recorded hash. Missing or corrupt artifacts are never repaired in place.
Cryptographic trust against `go.sum` or `GOSUMDB` remains layer 6.

### D-014: Extracted modules are not a package index

Shorthand package discovery no longer scans extracted `GOMODCACHE` directories.
Those directories can be incomplete and must not influence resolution even as
an index. Archive-backed shorthand discovery requires a validated archive
enumeration API; that API will be designed with layer 4 cache listing rather
than preserving the unsafe directory scan.

### D-015: Selector-aware private Git belongs to proxy-policy parity

Layer 3 carries the active build-list selection through the Go cache, scut
cache, and proxy paths. Applying that selection to direct private Git is
deferred to layer 5 because correct refs depend on repository roots, nested
module tag prefixes, semantic-import-version suffixes, replacements, and
GOPRIVATE/GONOPROXY policy. A simple `refs/tags/<version>` substitution would be
incorrect for several valid module layouts.

### D-016: Retain the authoritative Go build-list probe

The stack retains `go list -mod=readonly -m -json all` for authoritative MVS,
workspace, and replacement selection. It does not edit the active `go.mod`, but
the Go process may perform its own legitimate module-cache work. The strict
read-only guarantee applies to scut's in-process source fetchers and stores,
which never write, extract, repair, or chmod `GOMODCACHE`. Replacing the probe
with a complete in-process module loader is a distinct future project, not an
approximation added to this cache-safety stack.

### D-017: Cache commands are non-interactive and narrowly rooted

Invoking `remove`, `clean`, or `prune` is sufficient destructive intent. The
commands do not prompt because they are designed for agents and automation.
They resolve only beneath the operating-system-derived scut cache root, reject
malformed module targets, and never operate on `GOMODCACHE`.

### D-018: Latest aliases are evidence, not policy

When removal or pruning deletes the version named by a module's `latest` alias,
the alias is deleted. It is never retargeted to the highest remaining version,
because only a successful proxy `@latest` response can establish that mapping.

### D-019: Pruning uses immutable-entry publication time

Archive reads do not update cache metadata. `--older-than` therefore compares
the immutable archive's publication modification time, and `--max-size`
removes oldest entries first after age pruning. Cache accounting includes every
file under the owned root; if unassociated artifacts make the limit impossible,
prune fails visibly rather than deleting paths it cannot identify as canonical
entries. At least one policy is required, so bare `prune` cannot become an
accidental synonym for `clean`.

### D-020: Self-hash detects storage corruption, not source trust

New scut cache entries include an `h1:` content hash generated alongside the
validated ZIP before atomic publication. Inspection detects a missing or
mismatched sidecar. This protects cache storage integrity but does not prove
module authenticity; `go.sum`, checksum-database, and private-module trust
policy remain layer 6.

### D-021: Read Go environment policy without invoking Go

Scut resolves supported Go settings from the same three precedence layers as
the Go command: a non-empty process environment value, the user `go/env` file,
then `GOROOT/go.env`. `GOENV=off` disables only the user file. This preserves
`go env -w` configuration and toolchain defaults such as
`https://proxy.golang.org,direct` without adding another Go subprocess to the
documentation path.

### D-022: Remote fallback is one state machine

Proxy URLs, the implicit `noproxy` route, `direct`, and `off` are entries in one
ordered remote fetcher. A comma advances only after a not-found result; a pipe
advances after any result. Keeping this decision above HTTP proxy and Git
mechanisms prevents the generic source resolver from accidentally changing Go
fallback semantics.

### D-023: Private matching is source policy, not cache policy

`GONOPROXY` defaults to `GOPRIVATE` only when it is otherwise unset and is
matched against the resolved module identity before remote access. Existing
verified Go-cache and scut-cache archives remain reusable regardless of proxy
policy. `GONOSUMDB` is loaded into the same policy now but its trust decision is
enforced by layer 6.

### D-024: GOAUTH and Git authentication remain distinct boundaries

`GOAUTH` supplies prefix-scoped headers only to HTTPS go-import discovery and
module-proxy protocol requests, including its documented one-time 4xx retry.
Direct Git transport retains the existing environment token, GitHub CLI token,
and GitHub SSH fallback behavior; compatible Basic or Bearer credentials from
GOAUTH may be bridged to HTTPS Git, but `GOAUTH=off` does not disable Git's
separate authentication mechanisms.

### D-025: Insecure transport is direct-only

`GOINSECURE` can enable HTTP discovery fallback and relaxed TLS verification
only for matching modules on the `direct` route. It does not rewrite or weaken
explicit proxy URLs and does not alter checksum policy.

### D-026: Direct Git must also honor GOVCS

Although issue #54 initially named proxy and private-module variables,
implementing a real `direct` route makes `GOVCS` part of the same safety
boundary. Scut supports Git only and rejects a direct clone when the first
matching `GOVCS` rule disallows Git, using `GOPRIVATE` to classify the module as
public or private for the default rules.

## Open questions

No blocking questions are open for layer 1. Later layers must resolve these
before implementation reaches them:

1. Which configured build tags, beyond `GOOS` and `GOARCH`, should feed the
   documentation build context.
2. Whether checksum-database lookup should use the public default only when
   `GOSUMDB` is unset, matching the Go command, or require explicit opt-in for a
   documentation lookup tool.

## Progress log

- 2026-09-06: Diagnosed partial extracted-module writes as the cache corruption
  cause and reproduced the downstream Go command failure in an isolated cache.
- 2026-09-06: Agreed on a scut-owned archive cache and read-only Go-cache reuse.
- 2026-09-06: Created GitHub issues #50 through #56 for the seven stack layers.
- 2026-09-06: Initialized the seven local `gh stack` branches and checked out
  `gotools-cache/safety`. No branches were pushed.
- 2026-09-06: Identified the existing build-list `go list` subprocess as a
  separate cache-mutation boundary and recorded it as a later design decision.
- 2026-09-06: Layer 3 now reads only Go download-cache ZIPs with matching
  `.ziphash`, separates build-list selection from bytes, preserves exact
  selected versions across Go/scut/proxy archives, supports versioned archive
  replacements, and removes extracted-directory package indexing.
- 2026-09-06: Deferred selector-aware direct Git retrieval to layer 5 after
  tracing nested-module and semantic-import-version tag rules; documented the
  boundary instead of applying an incomplete tag-only fix.
- 2026-09-06: Layer 3 passes `./walle fmt`, `./walle test`, `./walle vet`,
  `./walle build`, and `./walle docs` with `GOTOOLCHAIN=go1.26.3`.
- 2026-09-06: User accepted retaining the authoritative read-only `go list`
  build-list probe and scoping the strict no-`GOMODCACHE`-mutation invariant to
  scut's in-process fetchers and stores.
- 2026-09-06: Implemented layer 4 cache inventory, self-hash sidecars, stable
  human/JSON inspection, non-interactive exact removal and clean, age/size
  pruning, total-byte accounting, latest-alias cleanup, and symlink-safe
  destructive path validation.
- 2026-09-06: Layer 4 passes `./walle fmt`, `./walle test`, `./walle vet`,
  `./walle build`, and `./walle docs` with `GOTOOLCHAIN=go1.26.3`; generated
  help covers every cache subcommand.
- 2026-09-06: Committed layer 4 locally as `0a8e08a`, rebased the remaining
  stack, and began layer 5 without pushing any branch.
- 2026-09-06: Layer 5 design review selected a single Go-compatible remote
  fallback state machine, direct reading of process/user/GOROOT Go environment
  policy, module-identity private matching, GOAUTH at HTTPS boundaries,
  direct-only GOINSECURE, and GOVCS enforcement for direct Git.
- 2026-09-06: Implemented layer 5 with Go environment-file precedence,
  comma/pipe/direct/off and file-proxy routing, private proxy bypass and
  overrides, GOAUTH helpers and 4xx retry, direct-only insecure transport,
  GOVCS enforcement, and selector-aware nested, semantic-version, pseudo-
  version, and replacement Git resolution.
- 2026-09-06: Layer 5 passes `./walle fmt`, `./walle test`, `./walle vet`,
  `./walle build`, and `./walle docs` with `GOTOOLCHAIN=go1.26.3`; a
  binary smoke check confirms `GOPROXY=off` returns the explicit policy error.
- 2026-09-06: Removed direct proxy and private-Git writes to `GOMODCACHE`, added
  the isolated downstream-Go regression, and updated the gotools CLI docs.
- 2026-09-06: Confirmed the repository test suite passes with its declared Go
  1.26.3 toolchain. The host-default Go 1.27.1 toolchain is incompatible with
  existing JSON-v2 calls and is not used for stack verification.
- 2026-09-06: Layer 1 passes `./walle fmt`, `./walle test`, `./walle vet`,
  `./walle build`, and `./walle docs` with `GOTOOLCHAIN=go1.26.3`.
- 2026-09-06: Amended layer 1 to capture `go list` stderr separately and restore
  Go-owned read-only permissions before temporary-cache cleanup; the full test
  suite exposed and verified both corrections.
- 2026-09-06: Implemented layer 2 with complete archive persistence, structural
  validation, atomic version directories, `latest` aliases, offline reads,
  best-effort cache writes, and immutable private Git revision capture.
- 2026-09-06: Layer 2 passes `./walle fmt`, `./walle test`, `./walle vet`,
  `./walle build`, and `./walle docs` with `GOTOOLCHAIN=go1.26.3`.
