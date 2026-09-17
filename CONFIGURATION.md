# Configuration

Priority (highest wins):

1. CLI flags
2. Environment variables (`AGENT_*`)
3. Project config (`./agent.toml`)
4. Global config (`~/.agent/config.toml`)
5. Defaults

## Example global config

```toml
[model]
provider = "openai"
model = "gpt-4"
api_key_env = "OPENAI_API_KEY"

[permissions]
mode = "ask"

[limits]
max_tool_calls = 50
max_parallel_tools = 4
```

## MCP servers

```toml
[mcp.servers.my-server]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem"]
enabled = true
```
