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
agent                 # interactive TUI mode
```

## TUI Interface

The agent runs an interactive terminal UI with the following navigation:

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Switch between form fields |
| `Enter` | Confirm/send/accept selection |
| `Esc` | Go back/cancel/exit |
| `↑` / `↓` | Navigate options/list items |
| `q` | Quit application |

**Screens:**
- **Banner** — Welcome screen with model info and quick commands
- **Input** — Message/command input with fuzzy autocomplete
- **Model Wizard** — Configure active model and provider
- **Provider Wizard** — Add/configure a new provider
- **Config Menu** — Settings (temperature, max tokens, etc.)

## CLI Commands

### Provider Management

```bash
# List all configured providers
agent provider list

# Add a new provider (interactive wizard)
agent provider add          # prompts for ID, URL, API key env

# Or specify all options
agent provider add --id my-azure --base-url "https://my.azure.openai.cn" --api-key-env AZURE_KEY

# Use a specific provider
agent provider use my-azure

# Remove a provider
agent provider remove my-azure
```

### Model Management

```bash
# List all models and show active configuration
agent model list

# Add a model (interactive wizard)
agent model add           # prompts for provider and model name

# Use a specific model
agent model use openai/gpt-4o
agent model use anthropic/claude-3.5-sonnet

# Show current active model
agent model show
```

### Other Commands

```bash
agent session list        # list saved sessions
agent session resume <id> # resume a previous conversation
agent mcp add <url>       # add MCP server
agent plugin list         # list installed plugins
agent skill install <url> # install a skill from URL
```

## Examples

```bash
# Start with a specific model
agent model use openai/gpt-4
agent                    # starts TUI with gpt-4

# Set up a new provider (Azure OpenAI)
agent provider add --id azure --base-url "https://my.openai.cn" --api-key-env AZURE_KEY
agent provider use azure
agent model use azure/gpt-4o

# Use mock provider for testing
agent model set mock mock-1
agent                    # runs without external API calls
```

## FAQ

**How do I add a provider?**

```sh
agent provider add
# Enter provider ID: my-openai
# Enter base URL: https://api.openai.com/v1
# Enter API key env: OPENAI_API_KEY
```

**How do I switch models?**

```sh
agent model list                    # see available models
agent model use openai/gpt-4o       # switch to gpt-4o
```

**Where is configuration stored?**

- Global: `~/.agent/config.toml`
- Project: `./agent.toml`

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
