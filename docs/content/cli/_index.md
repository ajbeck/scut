---

title: "CLI Reference"
description: "Command groups, configuration commands, hook entry points, and diagnostics."
weight: 30
---

The CLI reference summarizes scut commands and the files they read or write. Use `scut --help` and subcommand `--help` output for the authoritative flag list for your installed version.

Each reference page combines curated notes with help output generated from the current scut binary during the docs build. If the command tree changes, `mage docs` refreshes the generated blocks before Hugo renders the site.

## Generated command tree

{{< clihelp file="scut" command="scut --help" >}}
