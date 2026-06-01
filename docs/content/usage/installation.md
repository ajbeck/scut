---

title: "Installation"
description: "Install release binaries, pin versions, or build scut from source."
kicker: "Usage"
tags: ["install", "release"]
weight: 20
---

Scut ships as GitHub Release tarballs for macOS and Linux. The install script detects the current platform, downloads the matching artifact, verifies it against `checksums.txt`, and places the binary on `PATH`.

## Install script

{{< command >}}curl -fsSL https://install-scut.ajbeck.dev | sh{{< /command >}}

The default destination is `~/.local/bin/scut`.

Pin a release or choose another directory with installer flags:

{{< command >}}curl -fsSL https://install-scut.ajbeck.dev | sh -s -- --version v0.3.3 --bin-dir /usr/local/bin{{< /command >}}

| Flag                | Behavior                                                           |
| ------------------- | ------------------------------------------------------------------ |
| `--version VERSION` | Install a specific GitHub Release. The leading `v` is optional.    |
| `--bin-dir DIR`     | Install `scut` into the given directory instead of `~/.local/bin`. |

## Release assets

Each stable release publishes one tarball per platform plus a checksum manifest:

```text
scut-v0.3.3-darwin-amd64.tar.gz
scut-v0.3.3-darwin-arm64.tar.gz
scut-v0.3.3-linux-amd64.tar.gz
scut-v0.3.3-linux-arm64.tar.gz
checksums.txt
```

The tarballs contain one executable named `scut`.

{{< note type="info" icon="i" >}}
The release workflow deploys the documentation site from the same `main` commit used for the release, so hosted install instructions and published artifacts stay aligned.
{{< /note >}}

## Source installs

Go users can install from source:

```bash
go install github.com/ajbeck/scut@latest
```

For local development, use Mage so the JSON v2 experiment and build metadata are set consistently:

```bash
mage build
```

The local binary is written to `bin/scut`.
