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

## Version 0.2.1 startup intro

- Animated original SpherePrime mark and indeterminate progress are shown only during initial backend/chat readiness. There is no minimum display timer, and errors remain accessible.
- Real Electron acceptance checks the active animation, reduced-motion CSS and immediate dismissal on readiness; unit coverage verifies ready/failure do not render an overlay.

## Version 0.3.0 editor and interface acceptance

- Files open into a resizable editor pane beside the chat. Monaco and its workers are bundled locally; the editor is loaded on demand. Real Electron acceptance edits TypeScript with syntax colors, checks a blob worker, cancels a dirty tab close and saves to disk with Ctrl+S.
- Writes validate project boundaries and revisions, preserve UTF-8 BOM and refuse external changes. A temporary sibling is renamed into place only after the final revision check; saves in this application are serialized per path. This optimistic check does not provide a cross-process filesystem lock.
- Editor model cleanup is owned locally. A real Electron regression covers discard of inactive A while B has identical text, then reopening A; a delayed save regression verifies newer edits remain unsaved and the tab stays open.
- Dirty files prompt before window reload/exit; tab closing has Save, Discard and Cancel. Image and comparison tabs remain readonly.
- Chat search is revealed by a magnifier and clears when dismissed or switching project. Permission mode is an explicit described menu inside the composer toolbar. The Shift+Enter caption and project-opening loading banner are removed; the initial startup intro remains.

## Version 0.3.1 pane layout

- Open files use the order: chat list, chat, workspace files, editor. Hidden sidebar/workspace variants retain the editor at the right edge.
- Chat deletion uses a centered native modal with the chat title, initial focus on Cancel, Escape cancellation and duplicate-submit protection. Errors remain inside the dialog.

## Version 0.3.2 chat controls

- Recomputes the bottom position after reasoning blocks change height. Electron regression collapses a long reasoning block without leaving a false new-message indicator.
- Queue cards use stable IDs and offer Send Now. A real bundled-CLI scenario interrupts a held provider response and verifies the selected second entry runs before the first. Agent tests cover preserved attachments, identical prompt text, stale IDs and early-cancellation fallback.
- Pending approvals update independently from slow resource snapshots, with a one-second recovery poll during active runs. A lost-event regression stalls MCP loading and checks the permission still appears within 1.8 seconds. Active requests display a waiting-for-approval status.
- The composer exposes the main model's supported reasoning modes beside the model selector. A real CLI test saves High, verifies the configuration and returns it to Off.
- The pinned runtime advertises queue control; Desktop rejects runtimes without that capability. Existing string queue endpoints and legacy in-process folding remain compatible.

## Version 0.3.3 tool output and scrolling

- Tool calls and results are joined by call ID within user turns. One card shows the command or parameters and its output; unmatched output stays visible. Stored messages are unchanged. A finished argument stream alone does not mark tool execution complete.
- Manual wheel, touch, scrollbar and keyboard input cancel the virtualizer's pending auto-scroll. The return button is outside the scrolling content and targets the browser's actual end, including padding. Long-history coverage exercises reading during streaming, moving down and returning to the end.

## Version 0.3.4 integration settings

- MCP, LSP and Skills have separate Settings sections, available in the personal workspace before selecting a project. Inspector retains Files, Changes and Tasks.
- Server forms support scoped addition/editing, enable/disable and removal; existing secret fields are preserved through field-level writes. OAuth authorization, resources, prompt templates, diagnostics and channel controls remain available.
- The bundled CLI rebuilds its LSP catalog from current settings for start/discovery. Desktop stops existing language clients after edits or removal; catalog regressions cover adding, changing and disabling servers after manager creation.
- Skills supports additional folders, immediate discovery refresh, instruction viewing and reversible enable/disable controls.
- Prime options use language/notification selectors, switches and a context-compaction threshold. The generic key/JSON editor was removed.
- Voice input sits immediately beside Send/Stop, and the runtime version badge was removed from the sidebar. Version details remain in About.
- Unit coverage verifies OAuth boolean visibility and secret redaction. The real Electron acceptance verifies global MCP editing without losing environment/header secrets, a dotted MCP name, LSP creation, skill registration/toggling and the Prime language selector.

## Version 0.4.51 release updates and custom installer

- Stable complete Desktop releases are discovered after workspace readiness. The update controller deduplicates checks, automatically downloads a pinned release, forwards real progress, and offers Restart after download without a Later action.
- A real Electron updater fixture rejects a corrupted SHA-512 download and then accepts intact bytes. Controller tests exercise deduplication, guarded restart and download errors. The fixture never executes an installer.
- The custom frameless installer has its own controls and folder selector. Installation runs a SHA-256 verified silent payload and verifies application/runtime files before reporting completion. The visible UI only describes Prime Desktop.
- Windows command-line integration verifies the exact unquoted NSIS directory argument with spaces and Cyrillic text. A running Desktop blocks manual installation; the payload waits for a clean application exit without force-killing processes.
- The release workflow builds Desktop from the same CLI tag, including its runtime, and publishes the custom installer plus updater assets. Installer UI acceptance uses isolated fixtures and does not install into the user's profile.

## Public release requirements

- The stable update source is SpherePrime/CLI GitHub Releases. Publishing requires a stable tag containing the Desktop release job and source; no public release was created during local verification.
- The local Windows installer is not digitally signed. Production signing needs a certificate and release secrets.
- Clean-VM installation/upgrade/downgrade, physical microphone, sleep/wake and multi-process external-CLI database access need separate acceptance evidence.
- Backend event replay remains unsupported. Recovery uses current persistent message/session snapshots plus buffered live events and pending-request snapshots. No replay guarantee is advertised by capabilities.
- Existing-data upgrade/downgrade behavior needs installer-level acceptance before automatic deployment over established user installations.

The application is a functional local build. These remaining items prevent calling it a completed production release matching every criterion in the original comprehensive plan.
