# MCP

MCP (Model Context Protocol) servers are registered in config and connected at runtime.

## Transports

- `stdio` — spawn a process, communicate over stdio
- `http-sse` — connect to a running endpoint

## Discovery

Servers expose `tools`, `resources`, and `prompts`. The agent discovers them at connect time.

## Permissions

MCP tool calls pass through the same `PermissionEngine` as built-in tools.

## CLI

```sh
agent mcp list
agent mcp add <name> <command> [--args ...]
agent mcp remove <name>
agent mcp enable <name>
agent mcp disable <name>
agent mcp inspect <name>
```

## Example config

```toml
[mcp.servers.filesystem]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
enabled = true
```
