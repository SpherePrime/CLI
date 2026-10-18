# Prime Desktop

Windows desktop workspace for [Prime CLI](https://github.com/SpherePrime/CLI), with projects, persistent chats, streaming tool activity, approvals, model settings, Monaco file editing, diffs, MCP/LSP/Skills and optional voice input.

The bundled CLI starts with the desktop app and runs privately through a Windows named pipe. Chat, models and global MCP/Skills settings are available before opening a project; personal chats use a persistent app-managed workspace. No separate Go, Node.js or Prime installation is needed to run the packaged app. External tools such as Git, language servers and MCP runtimes remain optional dependencies of those features.

## Run from source

Use Node.js 24 and Go matching the sibling CLI repository's `go.mod`.

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

The installer and unpacked application are written to `release/` (the 0.3.4 delivery is under `release/0.3.4/`). Source builds require the CLI repository beside this repository. `runtime/manifest.json` records its exact revision and checksum.

## File editing

Open files from the inspector into editor tabs beside the chat. Monaco provides local syntax highlighting, line numbers, undo/redo and Ctrl+S. Saving checks the last-read revision before replacing a file; external changes require reloading. Unsaved tabs prompt before closing. Manual saves update the workspace and Git diff; CLI session history continues to describe agent tool edits. Editor code and workers are bundled locally and load only when needed.

## Data and updates

Queued messages appear above the composer. Send Now interrupts the current turn and runs the selected queued request, retaining its attachments and the other Desktop requests. Reasoning levels are available beside models that support them. Pending approvals update independently from resource loading and are recovered every second during active runs.

Prime owns session history and provider configuration. Desktop keeps window preferences and drafts in its Electron user data folder. Provider secrets are not returned to the renderer. Closing the window during an active run keeps the app in the tray; use its Exit action to stop and quit.

Runtime restarts mark unfinished runs as interrupted; they are never automatically repeated. Event reconnection reloads persistent history and pending requests rather than assuming server event replay.

This local build is unsigned. Automatic updates require a configured release feed. Desktop and its CLI ship as one versioned package; runtime hashes, protocol and required capabilities are checked on startup. Updating a separate external CLI does not replace the bundled runtime. See [release status](docs/release-checklist.md) for verification coverage and remaining release requirements.
