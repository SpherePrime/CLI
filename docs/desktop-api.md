# Desktop API

The existing `/v1` API remains compatible. Desktop clients should inspect `GET /v1/capabilities` before creating a workspace, keep its event subscription open, and fetch snapshots after reconnecting.

| Route | Result |
| --- | --- |
| `GET /v1/capabilities` | Protocol version and supported recovery features |
| `GET /v1/workspaces/{id}/pending` | `{permissions: [], question: null}` with current permission and question requests |
| `GET /v1/workspaces/{id}/runs` | Accepted runs ordered by acceptance time |
| `GET /v1/workspaces/{id}/runs/{run_id}` | Run status and authoritative terminal result; 404 if unknown |
| `GET /v1/workspaces/{id}/models/capabilities` | Configured enabled providers' model capabilities and unified reasoning modes |
| `POST /v1/workspaces/{id}/config/reasoning` | `{scope: 0, model_type: "large", mode: "high"}`; scope 0 is global, 1 is workspace |
| `GET /v1/workspaces/{id}/commands` | Existing custom command DTOs (`ID`, `Name`, `Content`, `Arguments`, `Skill`) |
| `GET /v1/workspaces/{id}/jobs` | Background shell jobs with originating `session_id`; optional `?session_id=...` filtering, with the last 64 KiB of each output stream |
| `GET /v1/workspaces/{id}/plugins` | Available first-party plugins and persisted enabled state |
| `GET /v1/workspaces/{id}/plugins/tasks` | Retained installation progress and errors |
| `POST /v1/workspaces/{id}/plugins/{name}/enable` | 202; install asynchronously, persist enabled state after successful installation |
| `POST /v1/workspaces/{id}/plugins/{name}/disable` | Cancel installation and persist disabled state |
| `POST /v1/workspaces/{id}/voice/transcribe` | `{mime_type: "audio/wav", audio_base64: "..."}` to `{text: "..."}` |

Send messages through the existing agent endpoint with a unique `run_id`. Repeating the same request with the same ID returns 202 without executing it again. A changed request with an existing ID returns 409. Acceptance is committed to the existing `prime.db` before dispatch and before the 202 response. The registry survives event disconnections, workspace removal, and process restart; `run_registry_persistence` is `sqlite`. Canonical project path and run ID form the storage key, including when projects share a data directory. Run states are `running`, `completed`, `failed`, `cancelled`, and `interrupted`; `running` includes an accepted queued request. Existing requests without `run_id` retain their previous behavior.

When a new server process opens the project, old unfinished records become `interrupted`. Normal workspace teardown also marks any remaining unfinished records `interrupted`. Retrying an interrupted run with its original ID only returns its known acceptance; it never executes the action again. The user must explicitly start a new task to repeat the work. Terminal results are persisted from the authoritative completion broker; if termination occurs before that write commits, recovery conservatively records interruption. Registry records are retained without automatic pruning. Storage remains subject to the existing database directory lock and SQLite WAL durability policy.

There is no event replay journal, event sequence watermark, or atomic combined snapshot. Service writes and event publication remain separate operations, so snapshot consistency is eventual. Reconnect by opening SSE before loading sessions, messages, resources, pending requests, and run snapshots, buffering live updates and reconciling records by ID. Do not repeat side effects under a new run ID when acceptance is uncertain.

Question answers use the existing `batch_request_id`. Question cancellation also accepts optional `{batch_request_id: "..."}`; supplying it prevents a delayed response from affecting a newer question. Exactly one concurrent answer or cancellation resolves a pending question.

Voice uploads require an enabled voice plugin and 16 kHz mono 16-bit PCM WAV, between 300 ms and 120 seconds, with an upper bound of 20 MiB. Browser WebM recordings must be converted before upload. Transcription reuses Prime's configured engine and language settings. Real microphone capture and hosted service availability require separate device and network validation.

The Windows named pipe grants access to the current Windows user SID. Authentication between separate Windows users has not been exercised in a second-user integration environment.

Workspace creation opts into MCP channels with `channels: ["serverName"]`. Only explicitly enabled servers advertising the channel capability can deliver messages. SSE uses an outer `mcp_channel_message` envelope whose payload is an event `{type, payload: {id, name, workspace_path, content, received_at}}`. The workspace path is canonicalized and delivery is limited to that workspace. Content is the escaped rendered channel element. Events have no session association and never start an agent automatically. Channel delivery has no replay or snapshot endpoint.

Before upgrading an existing SQLite schema, Prime saves a consistent `VACUUM INTO` backup as `prime-before-migration-<old>-to-<new>-*.db` in its data directory and validates it before running Goose. The backup includes committed WAL contents and remains available after migration. A database with a newer schema is rejected before migration; backup or validation failure also prevents migration.

Background shell ownership records the originating project and session independently from the command's working directory. This keeps commands launched in another directory visible in their originating project and allows filtering two concurrent chats correctly. Existing jobs without session ownership have an empty `session_id` and are omitted by a session-specific query.

Deferred: atomic snapshot and SSE replay, agent-task job snapshots beyond existing child sessions, additional MCP channel snapshots, and typed SDK wrappers for plugin mutations and voice uploads. Existing config endpoints continue to provide the rest of Prime's settings.
