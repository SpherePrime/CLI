# Native browser

Status: implementation and validation in progress.

Implemented in `desktop/src-tauri/src/browser/`:

- Native child Webviews with stable tab identifiers, a 12-tab limit, background tabs, selection, closure, bounds and visibility.
- Navigation, native Windows history including SPA entries, reload/stop, title and loading events, popup tabs, persistent browsing data and denied page permissions.
- Browser DOM snapshots, asynchronous JavaScript results through native callbacks, element selection, logging and browser tool definitions/dispatch.
- Windows WebView2 protocol adapter for screenshots and trusted keyboard/text input. Picker captures are registered in the shared attachment store.
- Loopback MCP endpoint with bearer authentication, body/header/batch/concurrency limits, browser-origin rejection, socket timeouts and shutdown; host startup passes its URL/token before loading the Go library.
- Background metadata and console-log collection.
- Windows native navigation failures and renderer failures are logged and propagated to waiting browser tools.
- Linux WebKitGTK adapter now uses native snapshots/Cairo PNG cropping, native editing commands for text, targeted GDK keyboard events, native back/forward lists and navigation/TLS/renderer failure hooks. Linux input does not fall back to JavaScript-generated keyboard events. Two Linux tests cover Unicode/key mapping and PNG region dimensions.
- Browser events target only the trusted main Webview. Child pages receive no application commands or capability grants.

Validation: Windows Cargo check and the first eight focused browser tests passed after host integration. All three embedded JavaScript files passed `node --check`. The initial tests cover URL/protocol handling, tab closure, HTTP/auth limits and the fourteen existing browser MCP tool names. Interactive Windows browser smoke checks are owned by the main migration task.

Linux validation: actual WSL Ubuntu Cargo check passed with WebKitGTK 4.1 version 2.52.6. Eleven browser/MCP unit tests passed. The separate GTK/WebKit display fixture passed under the normal WSL user: Unicode native text, native Enter keydown/keyup with DOM `isTrusted=true` and a real native PNG screenshot. The fixture waits for WebKit's asynchronous keyboard event queue before asserting key release. Runtime checks still pending: separate X11/Wayland sessions, background screenshots and multi-tab isolation. Windows changes remain behind their existing platform conditions.

Project MCP extension exposes all seven native terminal tools, seven launch tools, fourteen SSH tools, Git history/commit preview, GitHub reads, persistent working context and capability discovery. Workspace headers must decode to an already open project or its registered remote-engine local path; agent-supplied workspace/root overrides are rejected. Session metadata is carried separately. Terminal creation produces agent ownership; close/release require agent ownership; writes retain user handover checks. Context sources are validated against project file boundaries. Tests cover workspace binding and rejected overrides. Remote terminal/launch/Git/context-source operations explicitly reject local-cache execution while their remote adapters remain unavailable.

Remaining parity work: native screenshots/trusted keyboard/history/error adapters for macOS, Linux runtime validation, Chromium profile migration and idle-tab resource policy. Browser cookies use a separate persistent native profile; existing Electron browsing profiles are not imported. This MCP endpoint exposes browser tools; the old additional project/workbench MCP tools require their own host port. Unsupported native operations return explicit errors. This task does not claim full Electron feature parity.
