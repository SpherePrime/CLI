# Shared profile, hidden console and fast catalog

Status: fixed; verified on the release Windows build.

Root causes:

- The native host set `PRIME_GLOBAL_CONFIG`, `PRIME_GLOBAL_DATA` and `PRIME_CACHE_DIR` to its own empty app-data folders. The engine never read the user's `~/.config/prime`, so configured providers were missing.
- Workspaces were opened with a hashed `data_dir` inside the app-data folder, so project history in `<project>/.prime` was not visible.
- `migration::legacy_directory` looked for `Prime Desktop`; Electron stored data in `prime-desktop`. Nothing was imported.
- The Go engine runs inside a GUI-subsystem process without a console. Every console child it starts (git, shells, MCP servers, LSP) got its own visible Windows Terminal window.
- Startup waited for `catwalk.dwerty.local` and `hyper.dwerty.local` on every launch (~11 s DNS timeout) before the engine was ready.

Changes:

- Isolation is used only for tests and `PRIME_NATIVE_TEST_DATA` debug runs (`state::isolated_profile`). Release builds use the shared CLI profile and default project data directories.
- Personal chats use the legacy `prime-desktop/Chats` folder when it exists.
- Before the state loads, legacy `preferences.json`, `automations.json` and `launch.json` are imported once (`legacy-state-imported.json` marker). A fresh native profile takes the legacy preferences; a used one only fills empty values.
- `console::attach_hidden` attaches the process to a windowless console at startup, so all child processes inherit it instead of opening windows.
- `config.Providers` waits for the remote catalogs at most 2 s, then uses the cached or embedded copy; the refresh continues in the background and updates the cache.
- The git worktree root is found by looking for `.git` up the directory tree instead of running `git rev-parse`, which took 5 s per project open inside the host.
- Successful model discovery results are cached in-process for 5 minutes, so opening a workspace does not repeat the requests made at engine start.
- The renderer CSP meta tag left from Electron blocked `data:` fonts, `asset:`/`blob:` images and `ipc.localhost`; it now matches the Tauri policy.

Verification (release `prime-native.exe`, copy of the real profile): providers from `~/.config/prime` listed; UI ready in ~5 s (was ~20 s); two `!git` shell commands executed; a watcher polling every 150 ms saw no new console windows (the previous build opened ten). Go config/backend/desktopengine tests, 84 Rust library tests, typecheck and 19 renderer unit tests pass. `TestGlobFilesScopedPrefixMatchesUnscoped` in `internal/agent/tools` also fails on the previous commit.
