# Agent orchestration

## Behavior

Large tasks are split into independent assignments and delegated concurrently. The primary chat remains the coordinator and assembles the final result. Small tasks do not require delegation.

Each primary chat owns one team. The default limit is ten concurrently executing workers, excluding the primary chat. Further assignments wait in a queue. Lowering the limit does not interrupt active workers. Disabling orchestration blocks new launches and messages that would restart workers; active work may finish.

Workers have a name, assignment, role, status and persisted child session. They can exchange messages within their team, request follow-up work, and receive additional assignments without losing their conversation. Messages must not cross unrelated primary chats. Workers inherit the parent's permission mode.

The default executor can edit files. Research and planning roles keep their existing restrictions. Delegation must remain bounded; workers must not bypass the shared limit through nested launches.

Finished work is delivered back to the coordinator, including when the coordinator has ended its current turn. Delivery must not replay a result already consumed through the job tool.

## Desktop

An Agents tab in the right inspector lists workers for the current primary chat. Each row displays name, assignment and status. Selecting a worker opens its message history in the right panel without replacing the primary chat. The panel supports follow-up messages, stop, refresh and empty/error states. Existing approvals remain visible in the primary workspace.

Settings expose orchestration enabled/disabled and maximum concurrent workers. Defaults are enabled and ten. Values are persisted through the existing configuration system and remain compatible with updates.

Existing child sessions remain readable after restart; work that no longer has a live execution is shown truthfully rather than permanently running.

## Boundaries

Reuse the existing agent/background jobs, sessions, messages, config and native bridge. Do not introduce an external orchestrator, new dependency, unrelated UI changes, branches or worktrees.

Runtime options: `options.disable_multi_agent` (false by default), `options.max_concurrent_agents` (zero/unset resolves to ten).

Workspace calls:

- `agentWorkers {sessionId}` returns `{workers: AgentWorker[]}`.
- `agentWorkerMessages {sessionId, workerId}` returns `{messages: Message[]}` after team membership validation.
- `agentWorkerSend {sessionId, workerId, text}` returns an updated worker.
- `agentWorkerStop {sessionId, workerId}` stops the selected worker.

`AgentWorker` fields: `id`, `session_id`, `parent_session_id`, `root_session_id`, `name`, `agent`, `task`, `status`, `started_at`, `updated_at`. Times are Unix seconds. Statuses: queued, running, done, error, canceled, interrupted, idle.

The coordinator exposes an optional `AgentOrchestrator` interface with `AgentWorkers`, `SendAgentWorkerMessage`, and `StopAgentWorker`. This avoids forcing unrelated coordinator test doubles to implement desktop operations.

## Verification

Concurrency and cancellation tests prove limits, queue draining and bounded worker launches. Messaging tests prove team isolation, busy-session delivery and follow-up reuse. Runtime tests cover disabled mode and completion delivery. Desktop checks cover settings persistence, selecting a worker, streaming/poll recovery, follow-up, stop and primary-chat preservation. Run focused Go tests, race tests where supported, renderer typecheck/unit/build and isolated native acceptance.
