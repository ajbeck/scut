---

title: "CLI Reference"
description: "Root command, command groups, configuration commands, hook entry points, and diagnostics."
weight: 30
---

`scut` is the root command for the CLI. Use it directly for top-level actions such as setup, diagnostics, updates, and version output. Use command groups when you need a family of related subcommands, such as agent hooks, config installers, formatters, Go documentation lookup, logging maintenance, or MCP utilities.

Each reference page covers one command or command group. The generated help blocks are snapshots from the current scut binary and remain the authoritative flag listing for the documented command.

## Top-level commands

| Command                    | Purpose                                                         |
| -------------------------- | --------------------------------------------------------------- |
| [`scut version`](version/) | Print the installed scut version.                               |
| [`scut init`](init/)       | Install scut hook configuration for supported agents.           |
| [`scut doctor`](doctor/)   | Diagnose installed hook configuration.                          |
| [`scut update`](update/)   | Update scut when the install method supports automatic updates. |

## Command groups

Command groups contain subcommands. Run `scut <group> --help` to list the commands in a group, then use the linked reference page for details.

| Group                      | Purpose                                                     |
| -------------------------- | ----------------------------------------------------------- |
| [`scut claude`](claude/)   | Claude Code hooks, status line, and configuration commands. |
| [`scut codex`](codex/)     | Codex hooks and lifecycle integration commands.             |
| [`scut format`](format/)   | Source formatting commands.                                 |
| [`scut gotools`](gotools/) | Go tool-inspired commands for agents.                       |
| [`scut logging`](logging/) | scut log maintenance commands.                              |
| [`scut mcp`](mcp/)         | MCP utilities, including the AWS MCP proxy.                 |

## Generated help

{{< clihelp file="scut" command="scut --help" >}}
