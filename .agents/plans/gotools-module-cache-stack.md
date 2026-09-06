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

- Never write, extract, repair, chmod, or otherwise mutate `GOMODCACHE`.
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
| 2     | `gotools-cache/archive-store`      | [#51](https://github.com/ajbeck/scut/issues/51) | Planned                      | Add the scut-owned complete immutable archive store with atomic publication and concurrency safety. |
| 3     | `gotools-cache/go-cache-reader`    | [#52](https://github.com/ajbeck/scut/issues/52) | Planned                      | Reuse verified Go download-cache archives read-only and establish final source ordering.            |
| 4     | `gotools-cache/commands`           | [#53](https://github.com/ajbeck/scut/issues/53) | Planned                      | Add path, list, verify, remove, clean, and prune cache-management commands.                         |
| 5     | `gotools-resolution/proxy-policy`  | [#54](https://github.com/ajbeck/scut/issues/54) | Planned                      | Match Go proxy fallback, private-module, authentication, and transport policy.                      |
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

## Open questions

No blocking questions are open for layer 1. Later layers must resolve these
before implementation reaches them:

1. Whether cache `list` and `verify` JSON should use the repository-wide output
   envelope or a gotools-specific schema.
2. Whether explicit destructive cache commands need confirmation in interactive
   terminals, or whether the command invocation itself is sufficient intent.
3. Which configured build tags, beyond `GOOS` and `GOARCH`, should feed the
   documentation build context.
4. Whether checksum-database lookup should use the public default only when
   `GOSUMDB` is unset, matching the Go command, or require explicit opt-in for a
   documentation lookup tool.
5. Whether active build-list discovery may continue invoking `go list
   -mod=readonly -m -json all` against the user environment. The command cannot
   edit `go.mod`, but the Go tool may legitimately populate `GOMODCACHE`. PR 1
   prevents direct malformed writes by scut fetchers; a stronger process-level
   no-write guarantee would require isolating this probe or replacing it with an
   in-process build-list implementation.

## Progress log

- 2026-09-06: Diagnosed partial extracted-module writes as the cache corruption
  cause and reproduced the downstream Go command failure in an isolated cache.
- 2026-09-06: Agreed on a scut-owned archive cache and read-only Go-cache reuse.
- 2026-09-06: Created GitHub issues #50 through #56 for the seven stack layers.
- 2026-09-06: Initialized the seven local `gh stack` branches and checked out
  `gotools-cache/safety`. No branches were pushed.
- 2026-09-06: Identified the existing build-list `go list` subprocess as a
  separate cache-mutation boundary and recorded it as a later design decision.
- 2026-09-06: Removed direct proxy and private-Git writes to `GOMODCACHE`, added
  the isolated downstream-Go regression, and updated the gotools CLI docs.
- 2026-09-06: Confirmed the repository test suite passes with its declared Go
  1.26.3 toolchain. The host-default Go 1.27.1 toolchain is incompatible with
  existing JSON-v2 calls and is not used for stack verification.
- 2026-09-06: Layer 1 passes `./walle fmt`, `./walle test`, `./walle vet`,
  `./walle build`, and `./walle docs` with `GOTOOLCHAIN=go1.26.3`.
