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

## Reusable build workflow

The build workflow supports manual dispatch and workflow calls. In pull-request mode it verifies first, then builds platform artifacts. In release mode verification is skipped because the release commit has already landed on `main`.

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

{{< note type="warn" icon="!" >}}
If an exact release tag exists at a different commit, the workflow fails. Exact release tags are immutable.
{{< /note >}}
