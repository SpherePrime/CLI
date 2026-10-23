# Native teams and job center

Status: native implementation and eleven focused tests passed.

Original contracts: teams(workspaceId, request) lists, creates, removes, stops teams/tasks and retries finished tasks. Teams persist in teams.json version 1; limits are 100 teams, 20 tasks each and concurrency 1–6. Tasks use existing Go agent/init, sessions, agent and cancellation routes. The job center combines current/recent agent runs, automation runs, team tasks, launches and terminals, marks permission/question waits and routes stop to the owning service. It does not implement a second agent engine.

Implemented:

- Separate Rust team types with the original field names and camelCase storage, atomic bounded persistence with corrupt-file preservation and startup recovery of `starting` tasks.
- Serialized manager with project-bound lookup, per-team concurrency dispatch, claim persistence before dispatch (run id, session, workspace), status polling through the engine run registry, retry, stop with cancellation-failure retention as active tasks, and bounded eviction of finished teams at the 100-team limit.
- Native engine adapter opens the existing native project context and calls real Go agent/init, sessions, agent and run routes; remote roots return the explicit unsupported error.
- Job center snapshots open projects, Go runs/sessions/pending, automation runs, team tasks, launch processes and local terminals; deduplicates owned runs, keeps the one-day completed window, caches agent snapshots for 1500 ms and resolves stop against enumerated job kinds only.
- Registered `team` and `jobs` renderer commands behind the existing workspace binding; stop uses engine cancellation, automation cancel, team stopTask, launch stop and terminal close. Existing Go background shell jobs remain owned by the engine and are not duplicated as Rust processes.

Validation: eleven focused Rust tests passed (jobs status/time/label/filter/project-key mapping plus team creation under concurrency, claim persistence, polling completion and next-task start, dispatch failure and retry, stop with cancellation-failure retention, removal/limits, storage round trip and corrupt-file rejection). Full suite reached 73 passing tests; the pre-existing speech TTS fixture failure remains separate and unrelated. Live provider runs, renderer UI checks and remote team execution remain open.
