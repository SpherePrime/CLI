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
instantly. When something is still missing (a microphone recorder), the
menu shows a spinner with a progress bar and waits — a few MB on first
install, then it stays. The switch lives in `plugins.json` (global
config), per plugin:

```json
{ "voice": { "enabled": true } }
```

Toggling takes effect immediately: no `/restart_systems`, no restart.

## voice — microphone dictation

With the plugin on, <kbd>alt+v</kbd> (also <kbd>ctrl+shift+space</kbd>)
starts dictation anywhere in the TUI. The status bar shows `● REC` with a
timer; press the hotkey again to end the recording and get the transcript
at the cursor. Esc cancels without transcribing.

Transcription goes to Google's free Web Speech API, the same endpoint a
browser's speech recognition uses: no engine, no model, nothing to download.
The recording is a plain HTTPS POST, and the text comes back in a second or
two. A personal Google API key is optional (see `option voice api-key`);
without one the shared browser key is used and Google may throttle it.

Google's endpoint has no language detection of its own and rejects a request
that names no language, so Prime asks instead: `option voice language` takes
one BCP-47 tag, or a comma-separated list to choose from in order. A single tag
is a fixed choice and costs one request. A list is a search: Prime asks each
language in turn and keeps the answer written in the alphabet its own language
uses, which is what tells a model that understood the speech from one that
guessed, because a wrong model here is often the more confident of the two. A
confident answer in the right alphabet ends the search, so the common case
costs one request. Unset, or `auto`, searches `ru-RU` then `en-US`.

The alphabet check is a heuristic, and closely related languages are its weak
spot: on Russian speech a German model answers more confidently than the
Russian one, so a list mixing neighbours can pick wrong. Two well-separated
languages are reliable; three or more are worth checking.

```bash
option voice language ru-RU            # one fixed language, one request
option voice language ru-RU,en-US      # choose between two, in this order
option voice language auto             # the same search as leaving it unset
```

Engine options (`primerc`):

```bash
option voice engine auto        # auto, google, openai, command
option voice base-url https://api.openai.com/v1   # OpenAI-compatible endpoint
option voice api-key $OPENAI_API_KEY
option voice off                # keep the plugin installed but silent
```

## Adding a plugin

1. Implement the lifecycle in `internal/plugins/<name>.go`: `Install`
   (download/set up, idempotent, reports `Progress`), `Activate` (start
   background helpers), `Deactivate` (release them).
2. Register it in `plugins.All()`.
3. Add the menu strings `plugins.<name>.title` and `plugins.<name>.desc` to
   both catalogs in `internal/i18n/catalogs.go`. The menu picks it up
   automatically; state persists under `plugins.<name>` with no extra code.
