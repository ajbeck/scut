# Go 1.27 and Goldmark v2 stack

Status: active\
Last updated: 2026-09-07\
Repository: `ajbeck/scut`\
Trunk: `main`

## Objective

Upgrade scut to Go 1.27.1, retire the JSON v2 experiment plumbing now that
`encoding/json/v2` is stable, update the Go dependency graph, migrate Markdown
formatting to `goldmark-prettier-markdown/v2` and Goldmark v2, and make the
direct formatter command's file and configuration behavior explicit.

This document is the durable implementation plan and decision log. Update it
when a layer changes status, a design decision is made, new work is discovered,
or the stack shape changes.

## Invariants

- The Go 1.27 transition removes `GOEXPERIMENT=jsonv2` and every
  `goexperiment.jsonv2` build constraint; it does not replace them with another
  experiment flag.
- Existing `encoding/json/v2` and `encoding/json/jsontext` usage remains on the
  stable standard-library APIs, including migrating fallback fields from
  `json:",inline"` to `json:",embed"`.
- Run `go fix ./...` after the Go 1.27 transition and inspect its diff. Keep
  relevant modernizations in the toolchain layer; split or discard unrelated
  churn.
- Markdown front matter remains byte-for-byte preserved and unsafe or incomplete
  front matter continues to decline formatting.
- The Goldmark v2 parser enables the existing table, strikethrough, task-list,
  footnote, and definition-list syntax individually. Do not enable aggregate GFM
  linkification as an accidental policy change.
- Agent hooks retain stable default Markdown formatting behavior. Direct CLI
  options do not implicitly become hook configuration.
- Dependency upgrades target the latest stable release across module major
  versions. A new major receives a focused migration layer when its API or
  runtime behavior warrants independent review; prerelease-only majors are not
  treated as stable upgrades.
- Each layer updates its behavioral documentation and passes `./walle fmt`,
  `./walle test`, `./walle vet`, and `./walle build`; documentation layers also
  pass `./walle docs`.
- Do not push stack branches or open pull requests without explicit user
  approval.

## Stack

The stack is linear and listed bottom-to-top.

| Layer | Branch                         | Status              | Scope                                                                                                                                                                 |
| ----- | ------------------------------ | ------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | `go127-goldmark/toolchain`     | Implemented locally | Upgrade to Go 1.27.1, remove JSON experiment plumbing, migrate stable JSON tags, run `go fix ./...`, and update toolchain documentation.                              |
| 2     | `go127-goldmark/aws-proxy-v1`  | Implemented locally | Upgrade go-aws-mcp-proxy from v0.3.0 to v1.0.0 and independently verify the source-compatible but behaviorally substantial proxy release.                             |
| 3     | `go127-goldmark/renderer-v2`   | Implemented locally | Upgrade formatter and Goldmark module paths to v2, adopt the separated parse/render pipeline, preserve extension policy, and characterize intentional output changes. |
| 4     | `go127-goldmark/dependencies`  | Implemented locally | Refresh the completed direct-dependency graph and its transitive modules to their latest stable releases, then audit every graph change.                              |
| 5     | `go127-goldmark/file-contract` | Implemented locally | Make stdin a filter, make file arguments atomic in-place formatting, preserve modes, skip unchanged files, and add check mode.                                        |
| 6     | `go127-goldmark/options`       | Planned             | Expose prose-wrap, print-width, tab-width, and quote-style options for the direct Markdown command while retaining hook defaults.                                     |

## Layer 1 implementation plan

1. Set the module language/toolchain floor to Go 1.27.1 and let Go 1.27 tidy
   consolidate the direct dependency blocks.
2. Remove `GOEXPERIMENT=jsonv2` from Walle and remove all
   `//go:build goexperiment.jsonv2` constraints.
3. Change stable JSON fallback tags from `json:",inline"` to `json:",embed"`.
4. Run `go fix ./...` with Go 1.27.1, inspect every resulting edit, and retain
   only changes appropriate to the toolchain transition.
5. Update `AGENTS.md`, `CONTRIBUTING.md`, architecture documentation, and user
   installation requirements, including Go 1.27's macOS 13 minimum.
6. Run all Walle verification tasks and review the complete diff before commit.

## Layer 2 implementation plan

1. Upgrade `github.com/ajbeck/go-aws-mcp-proxy` from v0.3.0 to v1.0.0.
2. Retain the current proxy configuration defaults unless a v1 behavior requires
   an explicit compatibility setting.
3. Add or update focused integration tests for proxy startup, configuration, and
   the CLI-to-library boundary affected by v1's lazy connection, profile
   routing, retry, transport, and authentication changes.
4. Run module tidy, audit the graph change, update MCP documentation if behavior
   changes, and run all Walle verification tasks.

## Layer 3 implementation plan

1. Upgrade to `github.com/ajbeck/goldmark-prettier-markdown/v2@v2.0.0` and
   `github.com/yuin/goldmark/v2@v2.0.1`.
2. Replace `goldmark.New(...).Convert(...)` with an explicit Goldmark v2 parser,
   AST, and prettier renderer pipeline.
3. Register table, strikethrough, task-list, footnote, and definition-list
   parser extensions individually to preserve the current syntax policy.
4. Preserve the public byte formatter contract and front-matter behavior used by
   the CLI and both agent hooks.
5. Add regression coverage for Setext headings, nested emphasis, loose nested
   lists, footnotes, definition lists, representative MDX/raw HTML, and
   idempotence.
6. Remove the unused Goldmark v1 and wikilink graph through module tidy, update
   formatter documentation, and run all Walle verification tasks.

## Layer 4 implementation plan

1. Query every direct dependency for the latest stable release across major
   versions and confirm that any required module-path migrations are already
   represented in a focused lower layer.
2. Upgrade the full build list to the newest releases selected by those direct
   modules, including same-path transitive updates.
3. Run module tidy and review every direct, indirect, added, and removed module.
4. Explicitly record prerelease-only majors that were evaluated but excluded.
5. Run all Walle verification tasks and document any behavior-affecting
   dependency changes before commit.

## Layer 5 implementation plan

1. Preserve stdin-to-stdout filter behavior.
2. Make file arguments format files atomically in place instead of concatenating
   their contents on stdout.
3. Preserve file modes, skip unchanged files, honor ignore files, and retain
   `--force`.
4. Add `--check` with deterministic exit behavior and no writes.
5. Cover multiple files, partial failures, ignored files, unchanged files,
   permissions, and atomic replacement in tests and documentation.

## Layer 6 implementation plan

1. Add Markdown CLI flags for prose wrapping, print width, tab width, and quote
   style with validation and documented defaults.
2. Introduce an internal Markdown formatter configuration boundary while
   retaining `FormatMarkdown` as the stable default used by hooks.
3. Verify every option independently and in representative combinations.
4. Keep repository-level or hook-level persistent formatter configuration out of
   this layer unless separately designed and approved.

## Decisions

### D-001: Go 1.27.1 is the toolchain floor

Scut follows its existing exact patch-version convention and uses the latest
stable Go 1.27 patch. This also includes the first post-release fixes to
`encoding/json`.

### D-002: JSON v2 is no longer experimental

Go 1.27 promotes `encoding/json/v2` and `encoding/json/jsontext` to the standard
library. Walle, source files, tests, and contributor documentation must stop
requiring the old experiment flag and build tag.

### D-003: Go fix is audited, not blindly accepted

The Go 1.27 modernizer pass is part of layer 1. Its diff is reviewed before any
formatting or verification pass so unrelated rewrites do not obscure the
toolchain migration.

### D-004: Preserve parser policy explicitly

Goldmark v2's aggregate `GFMParser` includes linkification that scut does not
currently enable. Scut registers only its existing extensions so the dependency
upgrade does not silently broaden Markdown interpretation.

### D-005: Accept intentional v2 source-preservation improvements

The v2 renderer preserves Setext headings, canonicalizes combined emphasis such
as `***text***` to `_**text**_`, and correctly preserves loose nested-list
spacing. These are accepted as upstream formatter behavior and protected by
local characterization tests.

### D-006: CLI contract changes remain separate

The renderer migration keeps the existing `FormatMarkdown` caller contract.
File mutation semantics and exposed formatting options are separate layers so
reviewers can distinguish dependency output changes from CLI product decisions.

### D-007: Latest means latest stable across majors

The dependency audit does not stop at the current module path or major version.
Stable major upgrades are included, with their required import-path and API
migrations. Substantial migrations are isolated before the final graph refresh
so dependency churn does not conceal behavior changes.

### D-008: Major migrations receive focused layers

go-aws-mcp-proxy v1 keeps the `Config`, `RunOptions`, and `Run` surface scut uses
source-compatible, but the release makes substantial runtime changes. It gets a
dedicated layer. Goldmark and the prettier renderer move to `/v2` together in
the renderer layer because their API and output contracts are coupled.

### D-009: Prerelease-only majors are excluded

The go-git and go-billy v6 lines currently contain alpha releases only. The
stack records them as evaluated but retains the latest stable v5 releases unless
adopting prerelease dependencies is separately approved.

### D-010: Module-cache repair may require build-index invalidation

While resolving Goldmark v2, the installed scut v0.8.0 used by the go-doc skill
partially populated the shared module cache before the independent-cache release
was installed. Re-extracting the exact module directory restored its files, but
Go's build-cache module index continued reporting the restored packages as
missing. A fresh `GOCACHE` confirmed the stale index, and `go clean -cache`
repaired normal package resolution without clearing downloaded modules. The
current stack must use the repository-built scut for any further external doc
lookups until a release containing the independent-cache work is installed.

### D-011: File replacement is atomic per target

Named files produce no stdout. Each changed regular file is written to a
sibling temporary file, assigned the original mode, synchronized, closed, and
renamed over its target. Symbolic links are resolved before replacement so the
link remains intact. A multi-file invocation is intentionally not presented as
transactional: earlier successful files remain committed if a later file fails.

### D-012: Check mode applies to named files

`--check` runs formatting and ignore selection without writes and returns changed
paths once in argument order. It requires at least one named file. With no file
arguments, normal mode remains a stdin-to-stdout filter and passes declined
input through unchanged rather than swallowing it.

## Verification log

### Layer 1

- `go fix ./...` — passed; retained audited Go modernizer changes.
- `./walle fmt` — passed.
- `./walle test` — passed with the race detector.
- `./walle vet` — passed.
- `./walle build` — passed with host Go cache access.
- `./walle docs` — passed.

### Layer 2

- Compared the published v0.3.0 and v1.0.0 `proxy.Config`, `RunOptions`, and
  `Run` APIs; scut's existing integration is source-compatible.
- Exposed v1's allow-empty-tools, lazy-connect, and optional-auth controls while
  preserving nil defaults and making optional and skipped authentication
  mutually exclusive in the CLI.
- `./walle fmt` — passed.
- `./walle test` — passed with the race detector.
- `./walle vet` — passed.
- `./walle build` — passed with host Go cache access.
- `./walle docs` — passed and regenerated CLI help.

### Layer 3

- Upgraded `goldmark-prettier-markdown` to v2.0.0 and Goldmark to v2.0.1.
- Replaced the combined Goldmark v1 converter with the v2 parser, AST, and
  prettier renderer pipeline while preserving the existing extension policy.
- Added exact, idempotent output contracts for Setext headings, combined
  emphasis, loose nested lists, footnotes, definition lists, MDX, and raw HTML.
- Removed the Goldmark v1 and wikilink modules through `go mod tidy`.
- Repaired a partially populated Goldmark module directory and its stale Go
  build-cache module index; no downloaded module cache was cleared wholesale.
- `./walle fmt` — passed.
- `./walle test` — passed with the race detector.
- `./walle vet` — passed.
- `./walle build` — passed with host Go cache access.
- `./walle docs` — passed.

### Layer 4

- Confirmed every direct module is at its latest stable major and release.
  go-git and go-billy v6 were evaluated but excluded because only alpha releases
  exist.
- Updated every module in scut's production and test package closure. This
  includes the AWS SDK v2 family, smithy-go, ultraviolet, go-runewidth,
  knownhosts, crypto, test libraries, and their selected graph dependencies.
- Did not manually retain artificial pins for dependencies used only by
  upstream repositories' own tests; the final indirect requirements are exactly
  those retained by `go mod tidy` after the refresh.
- The production and test closure audit reports no remaining same-path updates.
- Cleared the complete Go build, module, test-result, and fuzz caches at the
  user's request, then redownloaded the graph from proxy/sumdb sources.
- `go mod verify` — passed against the cleanly redownloaded module cache.
- `./walle fmt` — passed.
- `./walle test` — passed with the race detector after the clean redownload.
- `./walle vet` — passed after the clean redownload.
- `./walle build` — passed after the clean redownload.

### Layer 5

- Named Go and Markdown files are formatted atomically in place and no longer
  concatenate their formatted contents on stdout.
- Preserved modes and symbolic links, skipped unchanged and ignored files, and
  retained per-file partial-failure semantics.
- Added `--check` for named files with no writes and deterministic changed-path
  reporting; stdin now passes declined content through unchanged.
- Covered ignores, force, modes, symlinks, duplicate check paths, successful
  checks, partial failures, rename failures, temp cleanup, and stdin decline in
  tests.
- End-to-end CLI smoke test confirmed empty stdout for file writes and a nonzero
  check result with the changed path.
- `./walle fmt` — passed.
- `./walle test` — passed with the race detector.
- `./walle vet` — passed.
- `./walle build` — passed.
- `./walle docs` — passed and regenerated CLI help.
