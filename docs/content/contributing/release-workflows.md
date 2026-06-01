---

title: "Release Workflows"
description: "Pull request checks, release tagging, GitHub Release assets, Homebrew tap updates, and Pages deployment."
kicker: "Contributing"
tags: ["release", "GitHub Actions"]
weight: 90
---

Scut uses GitHub Actions for pull requests, reusable builds, releases, Homebrew tap updates, and documentation deployment. Mage owns Go build, format, vet, and test commands.

## Pull requests

The pull request workflow runs formatting, vet, and tests through Mage. This keeps local and CI behavior aligned with the repo's JSON v2 build requirements.

PR runs use read-only permissions and a concurrency group based on workflow name and PR ref. New pushes cancel older runs for the same PR.

## Reusable build workflow

The build workflow supports manual dispatch and workflow calls. In pull-request mode it verifies first, then builds platform artifacts. In release mode verification is skipped because the release commit has already landed on `main`.

The reusable build has three conceptual phases:

1. `verify`: run `mage fmt`, `mage vet`, and `mage test`.
2. `build`: matrix build for `darwin/amd64`, `darwin/arm64`, `linux/amd64`, and `linux/arm64`.
3. `assemble`: download platform artifacts, build tarballs, write `checksums.txt`, and upload the `release-assets` artifact.

Release artifacts are named:

```text
scut-vM.m.p-darwin-amd64.tar.gz
scut-vM.m.p-darwin-arm64.tar.gz
scut-vM.m.p-linux-amd64.tar.gz
scut-vM.m.p-linux-arm64.tar.gz
checksums.txt
```

## Release workflow

The release workflow is manually dispatched with `releaseVersion` and `targetSha`. It checks out `main`, verifies that `targetSha` matches the current `origin/main`, creates the immutable `vM.m.p` tag, and force-updates the movable `vM.m` and `vM` tags.

After tagging, the workflow builds from the exact release tag, creates the GitHub Release, uploads assets, triggers the Homebrew tap update workflow, builds the Hugo documentation site, and deploys Pages.

Release job order:

1. `tag-release`: validate stable SemVer, require a full 40-character target SHA, require target SHA to match `origin/main`, create the exact tag, and update movable major/minor tags.
2. `build`: call the reusable build workflow in release mode with `ref` set to the exact tag.
3. `release`: create the GitHub Release and upload tarballs plus checksums.
4. `update-homebrew-tap`: mint a GitHub App installation token and dispatch the tap's formula update workflow.
5. `deploy-docs`: build and publish Hugo docs from the exact release commit.

{{< note type="warn" icon="!" >}}
If an exact release tag exists at a different commit, the workflow fails. Exact release tags are immutable.
{{< /note >}}

## Versioning

Release versions come from tags. `internal/version.Version` defaults to `v0.0.0-dev`; release builds override it through linker flags by setting `RELEASE_VERSION=vM.m.p`. Build metadata is injected separately.

Source installs such as `go install github.com/ajbeck/scut@vM.m.p` use Go build information as a fallback when linker flags are not set. Local Mage builds report the development version plus local timestamp metadata.

## Dependabot

Dependabot checks Go modules and GitHub Actions weekly. Updates are grouped by ecosystem so routine dependency churn lands as focused PRs.
