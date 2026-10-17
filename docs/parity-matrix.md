# Prime Desktop functional coverage

Source: CLI revision recorded in `runtime/manifest.json`; reviewed against workspace interface, API endpoints, command menu and config schema.

| CLI capability | Desktop location | Current evidence |
|---|---|---|
| Personal chats before project selection | Sidebar and composer | Real backend message, global MCP fixture and reload before folder open |
| Projects and context | Folder picker, recent projects; backend context discovery | Real Electron project-open test, Cyrillic path |
| Session create/list/history | Sidebar and conversation | Real CLI session and renderer-reload test |
| Rename/delete/pin | Session action menu | Implemented; pin is a Desktop preference |
| Streaming messages and tools | Conversation and tool details | Real provider fixture and real write tool test |
| Permission prompts | Inline approval cards | Real write approval creates a file after Allow |
| Structured questions | Inline single/multi/free text/yes-no form | Provider fixture question scenario |
| Queue and cancel | Composer queue and Stop | API integration; isolated backend cancellation tests |
| Plan and normal agent | Composer mode selector | Existing backend main-agent API; uses general/plan |
| YOLO | Persistent composer permission indicator | Existing skip-permissions API |
| Model/provider/reasoning | Anchored model list and provider/model editor | Go capability/reasoning tests; settings use backend config scopes |
| Local/custom endpoints | Provider form | Local fixture provider tested |
| Large/small model | Model picker tabs | Existing model_type API |
| Session tokens/cost/todos | Status bar and task panel | Backend values, never estimated by UI |
| Summarization | Chat actions → Summarize context | Existing summarize API |
| Direct shell | Composer prefix ! | Existing shell API, HTTP cancellation and main activity tracking |
| Custom commands | Chat actions → Prompt templates | GUI prompt templates under Chat actions; typed argument form and literal substitution |
| Attachments | Native file picker | Main stores chosen files and serializes real proto.Attachments |
| File tree and editing | Inspector Files → editor pane | Local Monaco, tabs, TS highlighting, Ctrl+S, unsaved-close guard; revision conflict and path boundary integration tests; 2 MiB limit |
| Session/Git diffs | Inspector Changes → readonly Monaco comparison | Full worktree status; renamed/deleted/untracked diff integration tests |
| MCP states/auth/resources/prompts | Inspector Resources, command preparation and advanced settings | Argument form and real backend prompt fetch; scoped opt-in channel delivery; Docker toggles |
| LSP diagnostics/start/stop | Inspector Resources | Existing LSP API |
| Skills catalog/read | Inspector Resources | Existing catalog/read API |
| Background shells | Inspector Tasks | Session-filtered backend jobs API |
| Plugins install/toggle/progress | Plugins dialog | Go plugin installation concurrency tests |
| Voice input | Alt+V / microphone button | WAV conversion + real backend transcription HTTP fixture; hardware check pending |
| Language | RU/EN preferences and native menus | Persisted language/reload Electron test; backend/provider content retains its language |
| Long conversations | Virtual message list | Real Electron test with 1,000 variable-height messages and fewer than 40 mounted rows |
| Theme and keyboard | Settings and shortcuts | Real Electron dark/light test and manual layout inspection |
| Close/tray | Main process lifecycle | Run activity regression tests; clean-VM lifecycle acceptance pending |
| Durable run deduplication | Bundled backend | SQLite reopen/no-redispatch tests |
| Local installer | Windows NSIS package | Packaged EXE passed full Electron/provider/tool/question scenario |
| CLI update | Separate external installation | Desktop never replaces a user's external CLI |

This table records actual scope, not a claim that every CLI menu item has a dedicated graphical control. All core settings are writable through the advanced configuration form; unsupported release scenarios are listed explicitly in the release checklist.
