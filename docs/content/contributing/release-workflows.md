---
title: "Release Workflows"
description: "Release Please pull requests, release assets, Homebrew tap updates, and Pages deployment."
kicker: "Contributing"
tags: ["release", "GitHub Actions"]
weight: 90
---
Scut uses GitHub Actions for pull requests, reusable builds, releases, Homebrew tap updates, and documentation deployment. Walle owns Go build, format, vet, and test commands. [Release Please](https://github.com/googleapis/release-please) owns release pull requests, `CHANGELOG.md`, exact release tags, and GitHub Releases.

## Pull requests

The pull request workflow runs formatting, vet, and tests through Walle. This keeps local and CI behavior aligned with the repository's Go 1.27 toolchain and build configuration.

PR runs use read-only permissions and a concurrency group based on workflow name and PR ref. New pushes cancel older runs for the same PR. The repository supports squash merges so the pull request title becomes the Conventional Commit that Release Please evaluates on `main`.

Use one of the repository's allowed Conventional Commit types in each pull request title. Release Please groups `patch` and standard `fix` entries under **Bug Fixes**, treats `feat` as a feature, and honors `!` or a `BREAKING CHANGE` footer as a breaking release signal. Documentation, refactoring, and maintenance entries are also included in generated release notes; test-only entries are hidden.

## Reusable build workflow

The build workflow supports manual dispatch and workflow calls. In pull-request mode it verifies first, then builds platform artifacts. In release mode verification is skipped because the release commit has already passed its pull-request checks and landed on `main`.

The reusable build has three conceptual phases:

1. `verify`: run `./walle fmt`, `./walle vet`, and `./walle test`.
2. `build`: matrix build for `darwin/amd64`, `darwin/arm64`, `linux/amd64`, and `linux/arm64`.
3. `assemble`: download platform artifacts, write `checksums.txt`, and upload the combined `release-assets` artifact.

Release artifacts are named:

```text
scut-vM.m.p-darwin-amd64.tar.gz
scut-vM.m.p-darwin-arm64.tar.gz
scut-vM.m.p-linux-amd64.tar.gz
scut-vM.m.p-linux-arm64.tar.gz
checksums.txt
```

## Release Please workflow

The Release workflow runs whenever `main` changes. Release Please reads Conventional Commits since the version recorded in `.release-please-manifest.json` and creates or updates one release pull request labelled `autorelease: pending`. The pull request proposes the next version and updates `CHANGELOG.md`; additional merges into `main` update the same pending release pull request.

Merging the release pull request causes the next Release workflow run to:

1. create the immutable `vM.m.p` tag and GitHub Release through Release Please;
2. build all four platform archives from that exact tag and generate `checksums.txt`;
3. attach the assets to the GitHub Release and update the movable `vM.m` and `vM` tags;
4. dispatch the separately managed `ajbeck/homebrew-tap` workflow, which opens a formula update pull request; and
5. build and deploy the documentation from the exact release commit.

The Homebrew repository owns formula review, testing, and merge. A successful scut release confirms that the formula update workflow was dispatched, not that its pull request was merged.

The version manifest is bootstrapped at the latest release that predates this automation. Do not hand-edit it during ordinary development; Release Please updates it in release pull requests.

## Release automation token

Release Please can fall back to the repository `GITHUB_TOKEN`, but GitHub suppresses workflow runs caused by changes made with that token. Configure the same GitHub App pattern used by Quorra so release pull requests receive ordinary CI runs:

- install the Release Please GitHub App on this repository with Contents, Issues, and Pull requests read/write permissions;
- set `RELEASE_PLEASE_CLIENT_ID` as a repository Actions variable; and
- set `RELEASE_PLEASE_APP_PRIVATE_KEY` as a repository Actions secret.

When the variable is absent, release pull request creation still works with `GITHUB_TOKEN`, but that pull request does not automatically trigger the pull-request workflow.

## Recover release assets

If building or uploading assets fails after Release Please creates the tag and GitHub Release, manually dispatch the Release workflow with `release_tag` set to the existing exact tag, for example `v0.9.0`. The recovery run validates that the release exists, rebuilds from its tag, and replaces its assets. It does not create another release, move the major/minor aliases, redispatch Homebrew, or redeploy documentation.

## Versioning

Release Please derives versions from Conventional Commits and records the current version in `.release-please-manifest.json`. `internal/version.Version` remains `v0.0.0-dev` in source; release builds override it through linker flags by setting `RELEASE_VERSION=vM.m.p`. Build metadata is injected separately.

Source installs such as `go install github.com/ajbeck/scut@vM.m.p` use Go build information as a fallback when linker flags are not set. Local Walle builds report the development version plus local timestamp metadata.

## Dependabot

Dependabot checks Go modules and GitHub Actions weekly. Updates are grouped by ecosystem so routine dependency churn lands as focused pull requests.
