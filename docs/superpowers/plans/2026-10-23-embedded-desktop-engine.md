# Embedded Desktop Engine Implementation Plan

Superseded by [Rust Desktop Migration](2026-10-23-rust-desktop-migration.md). The engine ABI and capability inventory remain relevant; Electron/Koffi hosting is no longer the target architecture.

Goal: ship Desktop as an independent application using an in-process Go engine, preserving the standalone CLI and existing Desktop functionality.

Architecture: retain the shared Go backend and tools. Build a versioned native library for Desktop, loaded through the existing Koffi dependency. Replace local process/socket transport with asynchronous native requests and bounded event polling. Keep the remote SSH transport separate. Do not rewrite tools in TypeScript or duplicate business logic.

Current findings: RuntimeSupervisor starts one bundled `prime server`; PrimeService creates workspaces and sessions through that server. This path does not spawn a CLI per chat. Removing the server process does not by itself eliminate retained histories or model memory. Electron renderers, browser processes, terminal commands, MCP servers and speech helpers remain where their functionality requires them. No zero-child-process claim.

## 1. Establish the engine boundary

- [x] Inspect RuntimeSupervisor, PrimeService and PrimeTransport lifecycle.
- [x] Introduce EngineTransport request/subscription interface; make PrimeService independent of socket transport.
- [ ] Record process counts and RSS/private memory for startup, 1/10/50 chats, concurrent tasks, closed projects, browser and voice workloads.
- [ ] Inventory all Desktop routes/events, cancellation, permission callbacks, configuration and shutdown hooks; map existing tests to each capability.

Files: desktop/src/main/prime/engineTransport.ts, service.ts, transport.ts; desktop/tests/integration.
Acceptance: unchanged socket implementation typechecks and existing workflows remain functional. Commit and push main.

## 2. Validate native hosting before migration

- [ ] Add cmd/desktop-engine as a separate Go library entry point without Cobra/CLI startup or process-global exit handling.
- [ ] Define versioned C ABI: version, create, request-start, response-read, cancel, event-read, release, shutdown. Use opaque handles, copied byte buffers, explicit lengths and ownership; never expose Go pointers.
- [ ] Validate Windows amd64 and Linux amd64 with Koffi async calls; then Windows/Linux arm64 native runners/toolchains.
- [ ] Keep all calls off the Electron main event loop. Use bounded per-request/event queues and cancellation, including abandoned subscriptions.
- [ ] Verify GC safety, repeated create/shutdown, error handling, Unicode and large payloads. Native crashes affect Desktop, so gate activation on these results.

Files: cmd/desktop-engine, internal/desktopengine, desktop/src/main/runtime/nativeLibrary.ts, scripts/build-desktop-engine.*.
Acceptance: native proof runs inside Electron with no `prime server` process and releases resources. If Go shared-library support/toolchains fail on a target, resolve hosting before changing production defaults; do not silently fall back to an external CLI.

## 3. Host the existing backend in process

- [ ] Extract backend lifecycle construction from server startup; preserve config initialization, global providers, permissions, database ownership and lazy agent creation.
- [ ] Dispatch existing protocol routes to the same handlers in memory, without a listening socket. Adapt streaming handlers to bounded incremental writers; do not buffer an entire SSE stream.
- [ ] Add NativeEngineTransport implementing EngineTransport; preserve request timeout, AbortSignal, reconnect semantics and event ordering.
- [ ] Handle pending permission responses and queued follow-ups without deadlocks while requests run asynchronously.
- [ ] Ensure one engine per Desktop instance, with isolated project/session state and explicit disposal.

Files: internal/server/server.go and routing, internal/backend, internal/desktopengine, desktop/src/main/prime/nativeTransport.ts.
Acceptance: route/event compatibility tests and concurrent sessions pass using the native host. CLI server behavior stays unchanged.

## 4. Preserve every capability

- [ ] Chat: streaming, reasoning, attachments/vision, pagination/search/export, title generation, model settings, tool access persistence and queued steering.
- [ ] Tools: filesystem, shell/background jobs and completion wakes, sudo secrets, skills, MCP, plans, teams and subagents.
- [ ] Desktop integrations: launch configurations, automations, Git/GitHub restore/revert, terminal handoff, window/browser permissions and multiple tabs.
- [ ] SSH: local engine executes remote operations through existing SSH/SFTP integrations; credentials stay outside model-visible payloads; saved sessions survive restart.
- [ ] Voice: input/output, device selection, cancellation and native model idle release. Inventory subprocess helpers separately rather than hiding them.
- [ ] Shutdown: cancel jobs/subscriptions, flush database, close projects, release models, join workers, then release ABI resources. Do not unload Go library while goroutines are alive.

Acceptance: focused end-to-end scenarios for each group, no duplicated coordinator or provider state, no secrets in events/logs. Commit each completed group to main.

## 5. Packaging and updates

- [ ] Build and checksum platform-specific native engine assets in CI; version ABI independently and reject incompatible libraries before load.
- [ ] Replace Desktop bundled prime executable with engine library and its required native dependencies. Keep CLI artifacts and prime update independent.
- [ ] Remove local RuntimeSupervisor spawn path only after native parity; retain SSH transport where needed.
- [ ] Update acquire-runtime, runtime verification, electron-builder resources, installers, Linux update restart and Windows locked-library replacement handling.
- [ ] Test clean installs, upgrades from existing releases, rollback to a previous installer, paths with spaces/non-ASCII and offline startup.

Acceptance: Windows/Linux amd64/arm64 packages launch without installed CLI and without a local prime child process; CLI installs still work separately.

## 6. Performance and release gate

- [ ] Repeat baseline workloads and compare total application-process memory, event latency, startup and scrolling. Include 30-60 minute active/idle cycles.
- [ ] Investigate retained objects/models using heap and Go runtime profiles; process consolidation is not a substitute for memory limits.
- [ ] Check native cancellation and crash behavior, concurrent project shutdown, updater restart and permission modes.
- [ ] Document measured results and remaining limits, review final diff, commit/push main. Publish a release only when explicitly authorized.

Status: first interface extraction implemented; native hosting, functionality parity, packaging and long-run measurements remain pending. The current production runtime is still process-based.
