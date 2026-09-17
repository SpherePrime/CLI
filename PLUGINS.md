# Plugins

Plugins extend the agent with new tools, commands, providers, and event hooks.

## Lifecycle

```
discover → load → initialize → activate → (deactivate → unload)
```

## Extension points

A plugin can add:

- tools (register in `ToolRegistry`)
- model providers
- slash commands
- event handlers
- configuration keys
- skills

## Versioning

Each plugin declares a minimum agent version. The loader rejects plugins whose requirements are not satisfied.

## CLI

```sh
agent plugin list
agent plugin install <source>
agent plugin remove <name>
agent plugin enable <name>
agent plugin disable <name>
agent plugin inspect <name>
```

## Creating a plugin

```sh
agent plugin install my-plugin
cd ~/.agent/plugins/my-plugin
cargo build
```

See `crates/agent-sdk` for the extension API.
