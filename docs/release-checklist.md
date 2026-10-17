# Local build verification

## Implemented and checked

- Strict TypeScript compilation and production Electron/Vite build.
- SSE UTF-8/chunk boundaries, nested credential redaction, path traversal/junction rejection, route validation and authoritative activity tracking tests.
- Real local named-pipe HTTP/SSE integration test.
- Electron launch with a bundled Prime process and Cyrillic workspace path.
- Real fixture-provider round trip through Prime, renderer reload, actual tool permission and file creation, theme switching.
- Go backend tests and vet for durable run storage, pending requests, plugins, voice upload, model reasoning, SDK and Windows pipe ACL.
- Runtime manifest ties the bundled binary to its exact Git revision and SHA-256.
- Brand geometry and colors taken from the public SpherePrime site; fonts and assets are local.
- The same full Electron scenario passed against `release/win-unpacked/PrimeDesktop.exe`, using its packaged assets and bundled runtime rather than development files.
- Structured custom-command arguments and MCP prompt preparation, explicit channel opt-in and workspace-scoped channel display are implemented.
- SQLite migration backup and newer-schema rejection passed Go tests. These do not substitute for a full installer upgrade test on a separate VM.
- Git worktree discovery covers modified, deleted, renamed and untracked files; integration checks verify HEAD comparisons without modifying workspace content. Files honor root .gitignore and .primeignore rules.
- English/Russian renderer and native tray/dialog/notification labels share a translation catalog. The persisted language switch passed real Electron reload acceptance. Provider content and backend diagnostics retain their original language.
- A real Electron component test scrolled 1,000 variable-height messages, verified fewer than 40 mounted message elements, retained the beginning while a new answer arrived and jumped back to the newest answer.

- Permission-start notifications never resolve or hide an outstanding approval. A hook-level Electron regression covers start → request → snapshot → explicit resolution; the final packaged full scenario passed twice consecutively.

## Version 0.2.0 interface acceptance

- Returning to personal chats after a remembered project and full process restart restores the persisted selected conversation; a subsequent real provider prompt passes.
- No folder required: the packaged CLI starts with Desktop, creates its private persistent chat workspace, loads global split configuration and connects resource events. Real Electron acceptance sends a message and restores it after reload before opening any project.
- Global MCP fixture in a separate mcp.json is visible in Resources before selecting a folder. Configured services remain visible even before connection status arrives. Existing user configuration is read by Prime rather than copied or reset.
- Model picker is anchored to the composer, searches and scrolls a 22-model catalog; the command palette and slash-command navigation are removed. Normal buttons expose chat actions and prompt templates.
- All dropdowns use themed listboxes with keyboard support, including controls inside native modal dialogs. Escape closes only the list and leaves Settings open. Numeric fields hide browser spinners.
- Provider editor shows every configured model and preserves its full metadata while editing context/response limits. A real backend test edits the second model and verifies all 22 entries and the unchanged first model.
- Settings use a larger responsive panel with readable fields; window chrome uses the SpherePrime titlebar and Windows overlay controls, including native drag/snap and controls available while a modal is open.

## Public release requirements

- No public Desktop repository or update-feed destination has been assigned. The updater reports an unavailable feed; no release was published.
- The local Windows installer is not digitally signed. Production signing needs a certificate and release secrets.
- Clean-VM installation/upgrade/downgrade, physical microphone, sleep/wake and multi-process external-CLI database access need separate acceptance evidence.
- Backend event replay remains unsupported. Recovery uses current persistent message/session snapshots plus buffered live events and pending-request snapshots. No replay guarantee is advertised by capabilities.
- Existing-data upgrade/downgrade behavior needs installer-level acceptance before automatic deployment over established user installations.

The application is a functional local build. These remaining items prevent calling it a completed production release matching every criterion in the original comprehensive plan.
