# Native teams and job center

Status: inventory complete; implementation started.

Original contracts: teams(workspaceId, request) lists, creates, removes, stops teams/tasks and retries finished tasks. Teams persist in teams.json version 1; limits are 100 teams, 20 tasks each and concurrency 1–6. Tasks use existing Go agent/init, sessions, agent, runs and cancellation routes. The job center combines current/recent agent runs, automation runs, team tasks, launches and terminals, marks permission/question waits and routes stop to the owning service. It does not implement a second agent engine.

Implementation plan:

1. Add separate Rust team types, atomic bounded persistence, engine adapter and serialized manager/scheduler modules. Keep project-bound lookup, native events, independent chats, concurrency, terminal statuses and retry behavior. Persist a claimed task before dispatch and reconcile in-flight work after restart without duplicate dispatch. Preserve cancellation failures as active tasks with visible errors.
2. Add a job-center module that snapshots open projects, Go runs/sessions/pending, automation/team activity, launch processes and local/SSH terminals. Deduplicate owned runs, retain the original one-day completed-run window and cache engine snapshots for 1500 ms. Resolve stop against a currently enumerated job instead of trusting client-supplied project paths.
3. Register original team MCP tools through the existing workspace binding and Go permission policy. Expose real errors for unavailable services or unsupported remote team execution.
4. Request root-owned state/commands/lifecycle wiring only after APIs are written. Add tests for scope isolation, persistence/recovery, concurrency, cancellation, retry and job ID routing. Run Cargo after the shared build queue is clear.

Integration needed: AppState teams Arc<TeamManager> and jobs JobCenter; TeamManager::load(directory), start(app), shutdown(); commands teams(workspaceId,request) and jobs(request). Browser workbench team adapters will be owned by this stage. No shared state/command/lifecycle source edits are part of this subtask.

Pending: implementation, focused tests, combined compile and UI smoke. Existing Go background shell jobs remain owned by the engine and its current routes/wake policy; they must not be duplicated as Rust processes.
