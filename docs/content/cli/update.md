---

title: "scut update"
description: "Update release-installed scut binaries or print package-manager guidance."
kicker: "CLI Reference"
tags: ["update", "install"]
weight: 25
---

`scut update` detects how the current binary appears to be installed and chooses the safest update path.

Script-installed release binaries, such as `~/.local/bin/scut`, can be updated in place. The command downloads the matching GitHub Release tarball, verifies it against `checksums.txt`, and replaces the current executable.

Homebrew and source-managed installs are not overwritten. For those installs, `scut update` prints the package-manager or source-build command to run.

```bash
scut update --dry-run
scut update
scut update v0.3.4
scut update --target-version v0.3.4
```

`--dry-run` reports the current version, target version, binary path, detected install method, and planned action without changing files.

## Generated help

{{< clihelp file="scut-update" command="scut update --help" >}}
