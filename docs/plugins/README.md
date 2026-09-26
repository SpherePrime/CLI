# Plugins

Prime has its own optional-features system, separate from MCP servers.
Plugins live inside the prime binary, are switched on from the plugins menu,
and download whatever they need at the moment you enable them. Nothing is
installed by default.

## The plugins menu

Open the commands dialog (`ctrl+p`), run **plugins**:

| Key | Action |
| --- | --- |
| enter | install or remove the highlighted plugin |
| ↑/↓ | choose |
| esc | close |

Enabling a plugin whose components are already on disk switches it on
instantly. When something is still missing (Whisper engine, model,
microphone recorder), the menu shows a spinner with a progress bar and
waits — a few hundred MB on first install, then it stays. The switch lives
in `plugins.json` (global config), per plugin:

```json
{ "voice": { "enabled": true } }
```

Toggling takes effect immediately: no `/restart_systems`, no restart.

## voice — microphone dictation

With the plugin on, <kbd>alt+v</kbd> (also <kbd>ctrl+shift+space</kbd>)
starts dictation anywhere in the TUI. The status bar shows `● REC` with a
timer; press the hotkey again to end the recording and get the transcript
at the cursor. Esc cancels without transcribing.

The engine is a resident local whisper.cpp server: it loads the model once
and answers in about a second instead of reloading hundreds of megabytes per
dictation. Prime starts it automatically when the plugin is enabled and keeps
it running for the whole session; deactivating the plugin stops it.

Engine options (`primerc`):

```bash
option voice language ru        # ISO 639-1; default: auto-detect
option voice model small        # or large-v3-turbo-q5_0, base, a ggml path
option voice engine auto        # auto, whispercpp, server, openai, command...
option voice base-url https://api.openai.com/v1   # remote transcription
option voice api-key $OPENAI_API_KEY
option voice off                # keep the plugin installed but silent
```

Where things live: the `voice` folder under the Prime data directory
(`prime dirs`) — `bin/` for the engine, `models/` for ggml weights,
`server.json`/`server.log` for the resident server.

## Adding a plugin

1. Implement the lifecycle in `internal/plugins/<name>.go`: `Install`
   (download/set up, idempotent, reports `Progress`), `Activate` (start
   background helpers), `Deactivate` (release them).
2. Register it in `plugins.All()`.
3. Add the menu strings `plugins.<name>.title` and `plugins.<name>.desc` to
   both catalogs in `internal/i18n/catalogs.go`. The menu picks it up
   automatically; state persists under `plugins.<name>` with no extra code.
