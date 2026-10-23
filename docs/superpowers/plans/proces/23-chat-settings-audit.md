# Chat model and settings audit

Status: code audit and focused checks passed. Live UI scenarios are recorded separately by the native smoke run.

The renderer model picker sends the selected provider/model with `sessionId` for an existing chat. The server routes this to `Backend.UpdateSessionModel`, and `ConfigStore.SetSessionModel` persists `session_models` in the workspace configuration. Other chats continue using their own selection or the preferred default.

The actual provider request follows the same selection: `coordinator.Run` resolves `sessionModel` before constructing provider options and passes it as `SessionAgentCall.ModelOverride`. `sessionAgent.Run` uses this override for prompt capabilities, message model/provider metadata, usage accounting and the `ModelProvider` callback. Merely changing the displayed label is not the request path.

Fixed during the audit:

- Clearing the final chat selection previously updated memory but skipped the disk write. The old selection returned after configuration reload. An empty `session_models` map is now persisted. The regression test failed before the change and passes after it, including another `Load`.
- Provider renaming previously updated preferred models and agent references but skipped chat selections. `renameProviderReferences` now includes `session_models`; a focused test verifies another provider remains unchanged.
- Live UI verification then exposed a second rename failure: selecting the provider in a chat created a workspace reference, which the global rename precheck rejected. Global saves now include the configuration stores of all open workspaces. Provider definitions and workspace model/agent/chat references are read and validated before writing, with sorted file locks and store mutation locks through reload. A write failure restores completed writes and reports any rollback errors. Workspace-owned definitions of the old ID or the proposed new ID remain explicit conflicts, preserving their settings and credentials.
- Sending a message previously put the global default in recent models even when the chat used an override. The renderer now records the chat selection first.

Reasoning is stored in native preferences under `reasoningBySession`, with `lastReasoningMode` as fallback. Sending explicitly reads current persisted preferences and forwards `reasoningMode`. The native settings store merges per-chat reasoning and permission maps and atomically writes preferences. The root audit also identified that native current-session selection only updated memory; its persistence fix belongs to the native host verification.

Checks run on Windows:

- `go test ./internal/config -run 'Test(SetSessionModel|RenameProviderReferences|SaveProvider)' -count=1` — passed.
- `go test ./internal/config ./internal/backend -run 'Test(SaveProvider|ProviderTransaction|GlobalProviderRename|RenameProviderReferences|SetSessionModel)' -count=1` — passed. The backend regression selects the provider in two open chats, renames it globally, and checks both in-memory selections and both persisted workspace files. Focused transaction checks cover write-failure rollback, removal of newly created transaction files and conflict rejection before any writes.
- `npm run typecheck` in `desktop` — passed.
- `git diff --check` — passed.

Remaining live verification scenarios:

1. Choose different models in two chats; send in each and inspect actual provider request bodies and saved message metadata.
2. Fully restart the app, reopen each chat, verify the selected model and reasoning and send again.
3. Select a chat without sending, restart, verify that it is the restored active chat.
4. Add a provider, restart, verify catalog and credentials are usable; rename it and verify existing chat selections.
5. Add an MCP stdio server; verify its tools, restart and verify reconnection. Add HTTP with OAuth enabled and exercise authorization with an appropriate test server.
6. Add an LSP server and verify file extensions, root markers, enable/disable and restart persistence. The settings implementation calls this integration `LSP`.

No live OAuth login or external provider credentials were exercised by this code audit.

The global rename transaction covers open workspace stores. It does not scan arbitrary closed projects for references. It provides rollback for reported write failures, not crash-atomic replacement of multiple files.
