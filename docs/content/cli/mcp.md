---

title: "scut mcp"
description: "MCP utility commands for agents."
kicker: "CLI Reference"
tags: ["MCP", "AWS"]
weight: 95
---
`scut mcp` provides Model Context Protocol utilities intended to be launched by MCP clients or agent configuration.

`scut mcp aws-proxy` starts a stdio MCP proxy for SigV4-protected AWS MCP endpoints. It mirrors the `aws-mcp-proxy` command-line surface while running through the scut binary.

The proxy can connect eagerly or defer the upstream connection until tools are requested. Authentication can be required (the default), skipped, or optional so unsigned requests are used only when credentials are unavailable. Empty upstream tool catalogs remain errors unless explicitly allowed.

## Generated help

{{< clihelp file="scut-mcp" command="scut mcp --help" >}}

{{< clihelp file="scut-mcp-aws-proxy" command="scut mcp aws-proxy --help" >}}
