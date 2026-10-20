package prompt

import (
	"fmt"

	"github.com/SpherePrime/CLI/internal/config"
)

func remoteWorkspaceContext(remote *config.RemoteWorkspace) string {
	return fmt.Sprintf(`

<remote_workspace>
The agent and model/provider requests run on the user's LOCAL computer, using its configured providers and credentials. SSH credentials are held by the desktop connection manager and are not available to you.
The TARGET project is REMOTE Linux over SSH. Host: %q. User: %q. Project root: %q.
All target file reads, searches, writes, downloads, shell commands, terminals, tests and Git operations must use the connected desktop SSH/project tools and execute on that remote host. The local working directory is only a proxy for local chat state, not the target project. Use Linux paths and resolve project-relative paths against the remote root.
Read the remote AGENTS.md and other applicable project instructions using ssh_read_file or ssh_exec before working; read nested remote instructions when needed. Remote Git status and project instructions have not been read automatically.
Local filesystem, shell, LSP, download and background execution tools are unavailable. Do not use local file paths, local execution, native sub-agents or local hooks to bypass this restriction. Browser/window interaction and provider/web requests remain local desktop capabilities; they do not change the remote target of project operations.
If the SSH connection or required remote tool is unavailable, report the connection/tool problem and stop dependent target operations. Never fall back to executing target operations on the local computer. Normal chat permission approvals and full-access settings still apply to desktop MCP tools.
</remote_workspace>
`, remote.Host, remote.User, remote.Root)
}
