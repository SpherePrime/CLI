# Prime Desktop

Windows and Linux desktop workspace for [Prime CLI](https://github.com/SpherePrime/CLI): projects, persistent chats, streaming tool activity, approvals, model settings, Monaco file editing, diffs, terminals, SSH projects, automations, MCP/LSP/Skills and voice.

The application is a Rust/Tauri host with a React renderer. The Prime engine is the Go backend compiled into a shared library (`prime_desktop_engine.dll` / `.so`) and loaded in-process; there is no separate CLI process. All child processes (WebView2, git, shells, MCP and LSP servers) run under the single `PrimeDesktop` process.

## Data

- Providers, models, MCP, LSP and skills: the shared CLI profile (`~/.config/prime`).
- Project chats: `<project>/.prime`. Personal chats: `%APPDATA%\prime-desktop\Chats` (Windows).
- Window preferences, drafts, automations and SSH profiles: the Tauri app data folder (`io.sphereprime.native-preview`). Secrets use the OS credential store.

## Run from source

Node.js 24, Go (version from the root `go.mod`), Rust stable and the Windows C++ build tools are required. From the repository root, `Prime.bat desktop` runs the app and `Prime.bat setup` builds the installer.

```powershell
npm ci
npm run dev
```

## Verify and package

```powershell
npm run typecheck
npm run test:unit
cargo test --manifest-path src-tauri/Cargo.toml --lib
npm run dist:win
npm run dist:linux
```

`npm run dist:win` builds the NSIS package (`src-tauri/target/release/bundle/nsis/`) and the custom installer `src-tauri/target/release/bundle/setup/PrimeDesktop-Setup-<version>.exe`.

## Installer

`installer/` is a small Tauri application with the Prime Desktop setup interface. The NSIS package is embedded into the executable and verified by SHA-256 before installation. Setup removes previous Electron and preview installations (application files only, user data is kept), installs into `%LOCALAPPDATA%\Programs\Prime Desktop` and downloads the voice model. Existing installations are updated in place.

Automatic updates use the signed Tauri updater feed (`latest.json`) published with stable GitHub releases.
