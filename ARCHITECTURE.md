# Architecture

Cargo workspace. Each capability lives in its own crate; the CLI composes them.

## Layout

```
crates/
  agent-cli/         clap entry point, subcommands, doctor/init/completion
  agent-core/        Core state, agent start/stop, session bootstrap
  agent-model/       ModelProvider trait + OpenAI/Anthropic/compatible/mock
  agent-tools/       ToolRegistry, ToolExecutor, parallel execution
  agent-filesystem/  read/write/edit + snapshot rollback engine
  agent-terminal/    shell runner, env-var protection
  agent-git/         status/diff/log/branch/checkout/add/commit
  agent-permissions/ ask/allow/deny engine, path limits, secret redaction, audit
  agent-context/     history + project detection + compaction
  agent-sessions/    JSONL persistence, resume
  agent-mcp/         stdio + HTTP/SSE client, tool/resource/prompt surfaces
  agent-skills/      SKILL.md discovery, enable/disable, global+project
  agent-plugins/     lifecycle manager (discover/load/initialize/activate/...)
  agent-events/      typed event bus + subscriber registry
  agent-config/      layered config: global/project/env/CLI/defaults
  agent-storage/     storage root, sessions, audit, snapshots
  agent-ui/          ratatui TUI widgets: frame, diff, spinner, slash palette
  agent-sdk/         public extension API surface
```

## Extension model

- **Model**: swap `ModelProvider` implementations via config
- **Tools**: register any `ToolExecutor` in the `ToolRegistry`
- **MCP**: servers appear as external tools through the same registry
- **Plugins**: `Plugin` trait drives lifecycle; they add tools, providers, commands, hooks
- **Skills**: prompt + tool + hook packages, discovered and applied by context

## Security

Every tool call passes through `PermissionEngine`:
- path restrictions, tool allow/deny lists, network scope
- secret redaction before tool output
- audit log in `~/.agent/audit/`
