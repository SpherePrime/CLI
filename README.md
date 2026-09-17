# Agent CLI

Production-ready, open-source AI coding agent written in Rust.

- Extensible: MCP servers, plugins, skills, custom tools
- Secure: permission layer (ask / allow / deny), secret redaction, audit log
- Fast: lazy loading, streaming responses, parallel tool execution
- Portable: single binary, no runtime dependencies

## Quick start

```sh
cargo build --workspace
cargo run -p agent-cli -- --help
```

```sh
agent init            # scaffold a project (agent.toml, starter skill)
agent doctor          # diagnose config, API keys, MCP, plugins
agent                 # interactive mode
agent model set mock mock-1   # configure a provider
```

## Build & test

```sh
cargo build --workspace
cargo test --workspace
```

## Configuration

Global: `~/.agent/config.toml`
Project: `./agent.toml`

Precedence: CLI flag > environment variable > project > global > default.

## Extension points

- **MCP**: `agent mcp add` registers a stdio or HTTP server
- **Skills**: markdown + tools + hooks, discovered from `~/.agent/skills/` or `./agent/skills/`
- **Plugins**: lifecycle-managed modules that add tools, commands, providers, and event handlers

See `docs/` for detailed architecture.
