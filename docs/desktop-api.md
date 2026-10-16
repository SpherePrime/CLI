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
| `GET /v1/workspaces/{id}/jobs` | Background shell jobs in this workspace, with the last 64 KiB of each output stream |
| `GET /v1/workspaces/{id}/plugins` | Available first-party plugins and persisted enabled state |
| `GET /v1/workspaces/{id}/plugins/tasks` | Retained installation progress and errors |
| `POST /v1/workspaces/{id}/plugins/{name}/enable` | 202; install asynchronously, persist enabled state after successful installation |
| `POST /v1/workspaces/{id}/plugins/{name}/disable` | Cancel installation and persist disabled state |
| `POST /v1/workspaces/{id}/voice/transcribe` | `{mime_type: "audio/wav", audio_base64: "..."}` to `{text: "..."}` |

Send messages through the existing agent endpoint with a unique `run_id`. Repeating the same request with the same ID returns 202 without executing it again. A changed request with an existing ID returns 409. The registry survives event disconnections and retains terminal results for the workspace lifetime; it is not durable across workspace removal or process restart. Run states are `running`, `completed`, `failed`, and `cancelled`; `running` includes an accepted queued request. Existing requests without `run_id` retain their previous behavior.

There is no event replay journal. Reconnect by opening SSE before loading sessions, messages, resources, pending requests, and run snapshots. Do not repeat side effects under a new run ID when acceptance is uncertain.

Question answers use the existing `batch_request_id`. Question cancellation also accepts optional `{batch_request_id: "..."}`; supplying it prevents a delayed response from affecting a newer question. Exactly one concurrent answer or cancellation resolves a pending question.

Voice uploads require an enabled voice plugin and 16 kHz mono 16-bit PCM WAV, between 300 ms and 120 seconds, with an upper bound of 20 MiB. Browser WebM recordings must be converted before upload. Transcription reuses Prime's configured engine and language settings. Real microphone capture and hosted service availability require separate device and network validation.

The Windows named pipe grants access to the current Windows user SID. Authentication between separate Windows users has not been exercised in a second-user integration environment.

Deferred: durable run history and SSE replay, agent-task job snapshots beyond existing child sessions, additional MCP channel snapshots, and typed SDK wrappers for plugin mutations and voice uploads. Existing config endpoints continue to provide the rest of Prime's settings.
