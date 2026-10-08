# Tool Description Listmcpresourcestool

Apply this reference only to its relevant task and within current user, project and mode instructions. Tool schemas determine supported parameters.

Lists available resources from configured MCP servers.
Call the configured MCP server by its discovered name. Prime returns a sorted textual list with resource titles, URIs and available metadata.

Usage examples:
- List resources from a specific server: `list_mcp_resources({ "mcp_name": "myserver" })`.
- `mcp_name` is required. Use search_mcp to discover servers rather than assuming an empty call lists all servers.
- Read a returned resource with `read_mcp_resource({ "mcp_name": "myserver", "uri": "the-returned-resource-uri" })`.
