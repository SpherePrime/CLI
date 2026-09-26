# Built-in MCP servers

Prime ships MCP servers inside the same binary. They are off by default; the
`/mcp` menu in the commands dialog installs them by writing a normal `mcp`
config section, which points back at `prime mcp serve <name>`. Nothing else
to download, and removing a server from the same menu stops it right away.

## The /mcp menu

Open the commands dialog (`ctrl+p`), type `mcp`:

| Command | What it does |
| --- | --- |
| Install MCP server: … | Writes the server's `mcp.<name>` section and connects its tools |
| Remove MCP server: … | Deletes the section and stops the running server |

Config writes make the backend reconcile MCP servers on its own, so tools
appear without restarting Prime. `prime mcp list` shows the same state from
the shell.

## voice — microphone dictation

Runs a resident local whisper.cpp server so short phrases transcribe in about
a second instead of reloading a half-gigabyte model per call. Tools (shown to
the agent with the `mcp_voice_` prefix):

| Tool | Purpose |
| --- | --- |
| `dictate` | Records until the speaker goes quiet (energy gate, ~0.7 s of trailing silence) or `max_seconds` is reached, then returns the transcript |
| `setup` | Downloads whisper.cpp and a ggml model into Prime's data dir, and a microphone recorder when none exists |
| `warm` | Starts the resident server now so the first `dictate` is fast |
| `status` | Recorders, engines, server state, and whether dictation is ready |
| `stop_server` | Frees the model's memory |

Typical use with an agent: ask "dictate me", the model calls `dictate`, you
speak, and the text comes back into the conversation. On first install just
tell the agent to set voice up; it calls `setup`.

### Engine options

The server reads `options.voice` from Prime's config — see
[../config/README.md#option-voice](../config/README.md).

```bash
option voice language ru
option voice model small
option voice base-url https://api.openai.com/v1   # remote transcription instead of local
```

### Where things live

- Engine and models: `prime dirs` data directory under `voice/bin` and
  `voice/models`
- Resident server state and log: `voice/server.json`, `voice/server.log`
- The server stops itself about 15 minutes after the last use

## Adding more first-party servers

1. Implement the server under `internal/mcps/<name>` with a
   `Serve(ctx) error` entry point using the vendored go-sdk
   (`mcp.NewServer`, `mcp.AddTool`, `mcp.StdioTransport`).
2. Register it in `internal/mcps/mcps.go` with a `BuildConfig` that points
   `prime mcp serve <name>` back at the current executable.
3. Add the localized title key `mcp.<name>.title` to both catalogs in
   `internal/i18n/catalogs.go`. The `/mcp` menu picks it up automatically.
