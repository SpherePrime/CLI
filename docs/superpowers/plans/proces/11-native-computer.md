# Native window viewing and control

Status: implementation written; combined native host compilation pending.

Baseline: `desktop/src/main/computer/` and `desktop/src/shared/computer.ts`. Existing behavior is Windows window listing, preview, per-workspace/per-chat view or control grants, bounded action logs, verified window identities, foreground-only input and Ctrl+Alt+Escape revocation. `window_request_access` is already governed by Go's normal chat confirmation/full-access policy; the native host must not add another approval or bypass that policy.

Implementation plan: separate Rust types, manager, Win32 window identity, capture, input and tool registration modules. Window identity includes PID creation time and a native window property to reject both PID and HWND reuse. Verify grants before and after captures, before input batches and at each action boundary. Non-Windows platforms must report unsupported behavior until native adapters exist.

Implemented: native window list/identity, native PrintWindow capture with bounded image sizes, capture concurrency and timeout, per-chat grants, view/control separation, foreground and pointer coverage checks, Unicode keyboard/mouse input, explicit focus, action logs and a dedicated Ctrl+Alt+Escape thread. Control grants/actions require the emergency stop to be active. All four original window MCP tools are registered. Three focused tests reject cross-chat/control upgrades, desktop-wide shortcuts, bad coordinates and action-scope overrides. Host module/state/lifecycle/command integration is owned by the main migration task.

Validation pending: native permission and identity tests, Windows compile, screenshot and bounded fixture-window input smoke, Linux unsupported-path compile checks, global emergency-stop registration and lifecycle checks.

Combined Windows host tests reached 50 passing tests; native window identity fixtures passed. One action-scope test found serde's unit-variant handling ignored extra fields on focus despite deny_unknown_fields. Focus now uses an empty struct variant so the same strict field validation applies to every action. The failed security test requires rerun; live capture/input/emergency-stop smoke remains pending.
