# Rust Desktop Migration

## Target architecture

Replace Electron and Node runtime services with a Rust/Tauri 2 application. Preserve the existing React/TypeScript/CSS renderer, assets, animations, Monaco and xterm frontend components. Preserve Go agent/tool implementations as an in-process native library with a versioned C ABI. Standalone CLI remains an independently packaged product.

This is Rust native hosting with an existing web frontend, not a rewrite of React components in Rust. The Go library must not launch a CLI server. WebView and user-command processes remain necessary; process count and memory must be measured rather than inferred from framework choice.

Production Electron remains available during development until the complete parity gate passes. The final migrated installer contains neither Electron nor a Node service nor a local CLI runtime executable. Do not ship placeholder implementations as completed features.

## Confirmed integration surface

The current preload exports DesktopApi through window.prime. Preserve this contract as the frontend boundary, replacing ipcRenderer with a typed Rust command/event bridge. Audit every exported method and DesktopEvent variant before enabling the new host.

| Current source | Rust replacement and acceptance |
| --- | --- |
| main/index.ts, preload/index.ts | Window lifecycle, single instance, tray, global shortcuts, startup splash, close/dirty confirmation, typed IPC and event cleanup |
| prime/service.ts, runtime/supervisor.ts | Shared engine sessions/workspaces, asynchronous FFI dispatch, cancellation, queueing, pending permissions and bounded events; no prime spawn |
| files/*, attachments/* | Filesystem scopes, revision conflict protection, trees, diff, clipboard images, drag/drop, attachment ownership and size limits |
| terminal/* | Native PTY implementation with resize, Unicode, output limits, handoff and shutdown; retain xterm renderer |
| remote/*, remoteRuntime/* | SSH/SFTP, known-host verification, credential vault, reconnect, port forwarding, remote commands, file transfers and history migration |
| browser/* | Embedded tab views, tab-targeted automation, DOM evaluation, clicks/typing, screenshots, element picker, logs, cookies, local file preview, downloads and lifecycle |
| computer/* | Per-platform window enumeration, captures, input, scoped permission checks and revocation |
| speech/* | Native voice/model hosting, microphone/speaker selection, streaming playback/cancel and model release; preserve renderer animation |
| github/* | History, diff, branches, PR/issues, git operations, restore/revert, backups and access policy |
| automations/*, teams/*, jobs/* | Persistence, scheduling, wakeups, launch profiles, teams, job output/cancel and application shutdown |
| settings/*, sudo/*, updater/installer | Existing settings migration, encrypted credentials, sudo secret isolation, signed updates, restart and rollback |

## Ordered implementation

### 1. Inventory and baseline

- [x] Inspect existing preload, browser views and engine launch boundary.
- [x] Select Rust host + retained renderer + shared Go engine; supersede Electron-native hosting plan.
- [ ] Enumerate DesktopApi methods/events in a machine-checked compatibility manifest.
- [ ] Capture UI reference screenshots and interaction scenarios for dark/light, narrow layout, panels, voice and editor.
- [ ] Record memory/process/startup/stream latency baselines for identical multi-chat workloads.

### 2. Feasibility gates before production replacement

- [ ] Build a Tauri shell loading the real renderer assets with isolated temporary data.
- [ ] Validate WebView2 on Windows and WebKitGTK on Linux: fonts, CSS effects, canvas/WebGL, Monaco workers, xterm, clipboard images, audio permissions and media playback.
- [ ] Prototype multiple embedded browser tabs with hidden-tab automation, screenshot/picker and persistent profiles on both platforms. Do not assume Chromium APIs exist in WebKitGTK.
- [ ] Validate Go shared-library compilation and Rust FFI on Windows/Linux amd64/arm64. Test repeated engine creation, async cancellation, buffer ownership and shutdown.
- [ ] Resolve any blocked feature at the platform adapter layer before calling this architecture feature-complete. Avoid iframe substitutes for arbitrary websites and hidden sidecar fallbacks.

### 3. Build the host and bridge

- [ ] Create desktop/src-tauri with separate Rust modules for lifecycle, commands, events, state, permissions and errors. Create a renderer host adapter implementing the current DesktopApi.
- [ ] Keep Node/Electron imports out of the renderer build; add an independent Vite build entry and scripts without replacing the current release build prematurely.
- [ ] Define engine ABI version/create/request/cancel/read-events/free/shutdown using opaque handles and explicit buffers; use Rust blocking workers rather than the UI thread.
- [ ] Reuse Go backend/protocol routing in memory. Preserve CLI startup separately. Never buffer an entire streaming response before delivering events.
- [ ] Scope privileged commands to the trusted application view. Remote browser pages must not receive filesystem, shell, secret or engine IPC access.

### 4. Port native services in functional groups

- [ ] Settings, paths, dialogs, filesystem and attachments; migrate existing data without destructive conversion.
- [ ] Engine sessions, streaming chat, tools, skills, models, permissions, steering, history paging/search/export and activity timing.
- [ ] PTY, background jobs, launch profiles, automations and teams.
- [ ] SSH/SFTP/forwarding, credentials, sudo and reconnect persistence.
- [ ] Git/GitHub operations with the existing backup, permission and restore semantics.
- [ ] Browser tabs/automation/picker and external window controls on supported platforms.
- [ ] Voice native inference, device selection, animations and resource lifecycle.
- [ ] Tray, shortcuts, OS integration, installers and updates.

Each group requires real functionality through the Rust host, focused behavior checks and a separate commit to main. A missing platform capability is an open item, not a silently disabled feature.

### 5. Packaging and migration

- [ ] Build native artifacts on Windows/Linux for amd64/arm64 with pinned toolchains and ABI manifests. Include required runtime libraries and WebView prerequisites.
- [ ] Preserve installed application identity, user data locations, projects, chat databases, preferences and saved SSH sessions.
- [ ] Explicitly migrate Electron safeStorage-encrypted secrets: Rust cannot assume it can read the old ciphertext. Validate each OS keyring/DPAPI path before removing the old host; never write plaintext migration files or expose secrets to the model.
- [ ] Bridge from the existing electron-updater feed to the new signed installer/update format. Test upgrade from the current installed release, not only clean Tauri installs.
- [ ] Change Prime.bat and CI only when native install/update tests pass. Preserve independent CLI packaging and prime update behavior.

### 6. Full parity and cutover

- [ ] Run the compatibility manifest against the native host; test concurrent chats/projects, queued tasks, approvals, restarts and failure recovery.
- [ ] Compare screenshot references and animation/audio behavior on both rendering engines. Fix deviations before replacing the default build.
- [ ] Repeat long-running memory measurements with equal workload and sum all application/WebView processes. Check idle release and foreground responsiveness.
- [ ] Verify installed Desktop works without CLI/Node installed and launches no local prime server.
- [ ] Remove Electron production dependencies, packaging and supervisor paths only after all capability groups pass. Archive legacy build instructions separately if needed.

## Current status

Architecture and migration inventory established. No Tauri application, Rust service ports or Go FFI host has been implemented yet. Existing EngineTransport extraction remains useful, but does not implement the Rust migration. Full visual and behavioral parity remains a required acceptance criterion, not a completed claim.

## References

- https://v2.tauri.app/reference/webview-versions/
- https://v2.tauri.app/reference/javascript/api/namespacewebview/
- https://docs.rs/tauri/latest/tauri/webview/struct.Webview.html
