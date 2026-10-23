# Native host boundary review

Status: reviewed with two actionable P2 findings. Fixes and verification remain pending.

## Findings

### P2: Validate traversal in the pathname, not search text

Location: `desktop/src-tauri/src/projects.rs:127`.

`call` applies `path.contains("..")` to the entire operation route, including its query string. The existing `messageSearch` route in `desktop/src/main/prime/operations.ts` uses `URLSearchParams`, which leaves periods unescaped. Searching chat history for `...` or `../src` consequently returns `INVALID_ROUTE`, even though the pathname stays within the workspace. Validate the parsed pathname and its normalized workspace prefix separately from query data. Preserve protection against encoded traversal.

Evidence: a Node reproduction using the existing route construction produced `sessions/example/messages?search=...` and `sessions/example/messages?search=..%2Fsrc`; both satisfy the native rejection predicate. The control query `hello` does not.

### P2: Confirm the event subscription before exposing a ready project

Location: `desktop/src-tauri/src/projects.rs:75-76` and `desktop/src-tauri/src/projects.rs:93-94`.

`open` sets the project connection to `ready` and returns after `Engine::subscribe` merely starts its reader thread. The Go ABI request returns after scheduling a handler goroutine (`internal/desktopengine/engine.go:95-112`), before that handler necessarily calls `SubscribeEvents` and `AttachClient` (`internal/server/proto.go:189-194`). An immediate initialization or send can therefore publish its first events before this project's event subscriber exists. The reader ignores successful headers rather than acknowledging readiness. The retained Electron service explicitly waits for stream readiness before returning the project (`desktop/src/main/prime/service.ts:114-131`). Add a bounded subscription handshake based on successful headers and propagate initial failure, retiring the client on failure before reporting the project open.

Evidence: static control-flow review of both sides of the ABI and the existing service's readiness barrier. This timing issue was not reproduced in a running UI during this review.

## Check scope

- Reviewed `engine.rs`, `engine_resource.rs`, `commands.rs`, `permissions.rs`, `events.rs`, `lifecycle.rs`, `state.rs`, `projects.rs`, `capabilities/main.json`, `nativeBridge.ts`, and `windowChrome.tsx`.
- Cross-checked the retained C ABI and Go request lifecycle in `cmd/desktop-engine` and `internal/desktopengine`, server event subscription, window command authorization, event batching, resource verification, and existing renderer operation routes.
- Confirmed source-level main-webview and trusted-origin checks for desktop commands, main-only capability assignment and event emission, C allocation release, request cancellation on release, response and event buffering limits, and deliberate retention of the loaded Go library.
- Disk attachment changes in progress, intentionally unsupported preview features, production migration, and unrelated native service implementations were excluded.
- No Cargo build, automated test suite, browser security probe, or UI smoke test was run in this review. The only executable reproduction was the Node route/query check described above. No production-readiness or complete shutdown-boundedness claim is made.
- Branch checked: `main`. Only this report was written; no commit was created.
