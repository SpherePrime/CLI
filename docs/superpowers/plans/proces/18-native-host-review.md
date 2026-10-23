# Native host boundary review

Status: both P2 findings fixed and covered by tests.

## Findings

### P2: Validate traversal in the pathname, not search text — fixed

`desktop/src-tauri/src/projects.rs`. Route validation now splits the pathname from the query: traversal checks apply to the parsed path and the normalized workspace prefix, while `messageSearch` values such as `...` or `../src` stay query data. The regression test `search_query_dots_are_data_and_paths_remain_scoped` covers dot-only searches, encoded traversal and workspace-scope escapes.

### P2: Confirm the event subscription before exposing a ready project — fixed

`desktop/src-tauri/src/engine.rs`. `subscribe` waits for a bounded handshake: the subscription is only registered after the engine returns successful response headers, and a failed or timed-out handshake cancels the request, releases the handle and propagates the error before the project is published as ready. The reader thread starts afterwards.

## Check scope

- Reviewed `engine.rs`, `engine_resource.rs`, `commands.rs`, `permissions.rs`, `events.rs`, `lifecycle.rs`, `state.rs`, `projects.rs`, `capabilities/main.json`, `nativeBridge.ts`, and `windowChrome.tsx`.
- Cross-checked the retained C ABI and Go request lifecycle in `cmd/desktop-engine` and `internal/desktopengine`, server event subscription, window command authorization, event batching, resource verification, and existing renderer operation routes.
- Confirmed source-level main-webview and trusted-origin checks for desktop commands, main-only capability assignment and event emission, C allocation release, request cancellation on release, response and event buffering limits, and deliberate retention of the loaded Go library.
- Disk attachment changes in progress, intentionally unsupported preview features, production migration, and unrelated native service implementations were excluded.
- Both fixes were validated by focused Rust tests in the combined host suite. No production-readiness or complete shutdown-boundedness claim is made beyond these checks.
