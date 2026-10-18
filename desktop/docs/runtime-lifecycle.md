## Desktop startup and CLI updates

The CLI is bundled inside Desktop and starts hidden with the application. A personal-chat workspace opens automatically; no project folder or separate CLI launch is required for chat, model/provider selection, global MCP or skills. Opening a project adds that project's context. Global configuration remains owned by Prime, including split files such as ~/.config/prime/mcp.json and providers.json.

Runtime and Desktop updates form one package. The manifest pins the CLI revision, checksum and protocol version; startup verifies compatibility and required capabilities. A separately updated external CLI is independent. This build does not silently download or swap a runtime because no release/update feed has been assigned.
