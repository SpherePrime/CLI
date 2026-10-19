# Prime Desktop

Windows desktop workspace for [Prime CLI](https://github.com/SpherePrime/CLI), with projects, persistent chats, streaming tool activity, approvals, model settings, Monaco file editing, diffs, MCP/LSP/Skills and optional voice input.

The bundled CLI starts with the desktop app and runs privately through a Windows named pipe. Chat, models and global MCP/Skills settings are available before opening a project; personal chats use a persistent app-managed workspace. No separate Go, Node.js or Prime installation is needed to run the packaged app. External tools such as Git, language servers and MCP runtimes remain optional dependencies of those features.

## Run from source

Use Node.js 24 and Go matching the repository root `go.mod`. From the repository root, `Prime.bat desktop` runs the app and `Prime.bat setup` builds the setup.

```powershell
npm ci
node scripts/acquire-runtime.mjs
npm run dev
```

## Verify and package

```powershell
npm run typecheck
npm run test:unit
npm run test:integration
npm run build
npm run test:e2e
npm run runtime:verify
npm run dist:win
```

The installer and unpacked application are written to `release/<version>/`. The bundled CLI is built from the repository root; `runtime/manifest.json` records its exact revision and checksum. Stable GitHub releases include `PrimeSetup-<version>.exe` and Desktop update files.

## Built-in browser

The Workspace panel has a Browser tab. Prime uses the same browser through its `browser` tools: it opens pages and local files, reads them, clicks, types and takes screenshots. The page runs in an isolated session without access to Prime Desktop.

## File editing

Open files from the inspector into editor tabs beside the chat. Monaco provides local syntax highlighting, line numbers, undo/redo and Ctrl+S. Saving checks the last-read revision before replacing a file; external changes require reloading. Unsaved tabs prompt before closing. Manual saves update the workspace and Git diff; CLI session history continues to describe agent tool edits. Editor code and workers are bundled locally and load only when needed.

## Terminal

Open Project Tools or press Ctrl+` for project-scoped PowerShell tabs. Resize or expand the panel, copy output, and explicitly hand input control to the agent. Terminals keep running when hidden or while switching projects. Closing a tab stops its process tree; quitting stops every terminal. Active terminals block update restart. Native terminal dependencies ship with the application.

## Data and updates

Queued messages appear above the composer. Send Now interrupts the current turn and runs the selected queued request, retaining its attachments and the other Desktop requests. Reasoning levels are available beside models that support them. Pending approvals update independently from resource loading and are recovered every second during active runs.

Prime owns session history and provider configuration. Desktop keeps window preferences and drafts in its Electron user data folder. Provider secrets are not returned to the renderer. Closing the window during an active run keeps the app in the tray; use its Exit action to stop and quit.

Runtime restarts mark unfinished runs as interrupted; they are never automatically repeated. Event reconnection reloads persistent history and pending requests rather than assuming server event replay.

Updates are discovered in stable SpherePrime/CLI GitHub Releases after startup. Desktop release assets are required; incomplete or CLI-only releases are skipped. Downloads are automatic and checksum verified. Restart is explicit, with active-work and unsaved-file guards. Desktop and its runtime ship as one versioned package. See [release status](docs/release-checklist.md) for verification coverage and remaining release requirements.


## Release and installer

`PrimeSetup-<version>.exe` is the complete custom installer. It contains the full offline application payload, uses a frameless interface and verifies installed runtime files before reporting success. The separate `PrimeDesktop-Update-<version>.exe` and `latest.yml` assets are used for automatic updates.

Build the main app and NSIS update payload first, then run `npm run dist:installer`. The payload must be in `release/<version>/`. The custom installer appears in the same directory.

The CLI release workflow builds Windows Desktop after the CLI release, assigns the stable tag version, and attaches both installers, the update blockmap and `latest.yml`.
