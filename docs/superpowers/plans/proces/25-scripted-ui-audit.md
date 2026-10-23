# Scripted native UI audit

Status: integration checks in progress; final Windows artifact verification pending.

The dev application is driven through Playwright and WebView2 CDP. User actions use the rendered controls, not direct DesktopApi calls. A local compatible provider records the requested model and URL. The runner captures step screenshots, complete Playwright traces, application logs, failed DOM snapshots and JSON results. Test data, projects, credentials and profiles are isolated from the installed application.

Verified scenarios include separate chat models and actual provider requests, reasoning, favorites, access modes, theme/density, provider creation, MCP HTTP initialization and tool listing, OAuth option editing, disabled LSP configuration, settings tabs, reopening an older active chat, drafts and settings after a full application restart. Project tool tabs preserve their dock height; browser tabs retain their URLs. External provider account authentication is not exercised by the local fixtures.

Findings fixed during the audit: native active-session selection was not persisted; a draft could be lost before its delayed save and was not initially restored when the composer mounted with an existing session; duplicate skill paths produced duplicate React keys; native component information incorrectly showed a named-pipe connection. The window now waits for draft persistence before closing, including native close requests. Update installation also flushes drafts first.

The added-provider rename scenario exposed a remaining cross-scope reference guard; its regression fix is being validated before the final run. The isolated project fixture also exercises the actual file editor and Git history without modifying the working repository.

Current verification: TypeScript and 19 renderer unit tests passed; 82 compiled native library tests passed with four external fixtures ignored; Go config, backend and embedded-engine packages passed. Linux native library compilation passed after adding the updater. NSIS compilation is running on Windows.

Limitations: signed native update download/install/restart requires a verified public key and published feed, neither of which is configured in the local unsigned installer. This build must not claim a successful signed upgrade. Linux installer execution, ARM64 runtime checks and migration of every legacy data/secret format remain separate gates. The retained legacy source is not part of the native Windows installer.

Artifacts are generated under `desktop/tests/e2e/.workbench-ui-audit/` and intentionally ignored by Git. Run from `desktop/` with a prepared debug host and the dev renderer listening on 127.0.0.1:5173: `node tests/fixtures/native-ui-audit.mjs`.
