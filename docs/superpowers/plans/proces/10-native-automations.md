# Native automations

Status: native local scheduler implementation and eight focused Windows tests passed.

Scope: existing automations.json compatibility, interval/daily/weekly local-time scheduling, explicit DST handling, persisted claims and run history, cancellation and startup recovery, bounded concurrency, filesChanged/runFailed cooldown and deduplication, and native Go engine dispatch through the existing project/session context.

Implemented: existing storage schema with atomic writes and corrupt-file preservation, 200 inactive history records, one active run per rule and four concurrent runs globally, persisted claims and session IDs before dispatch, interval/daily/weekly schedules, native recursive filesystem events with ignore rules, cooldown/deduplication/self-run suppression, registry recovery and cancellation. The production adapter opens the existing native project context and calls real Go session, agent and run-registry routes.

Validation: all eight focused Rust tests passed in 2.45 seconds. Checks cover DST spring gaps and fall folds, weekly schedules, actual local task processes through an adapter fixture, claim/session/history persistence, cancellation, concurrency and cross-root restrictions, recovery of unsent claims and previous process runs, cooldown/deduplication/self-event suppression, corrupt-file preservation, and native filesystem change notifications with ignored generated directories.

The task fixture starts real local processes to verify scheduler behavior; it does not claim to validate a live model-provider run through the production adapter. Native provider/UI integration and Linux/macOS checks remain open. Host state/command/lifecycle wiring is coordinated with the host implementation. Failure-event routing from the engine subscription is a separate integration hook.

Shutdown exposes stop() to prevent new claims before closing the engine, then shutdown() joins the scheduler and releases monitors. Running records remain recoverable through the Go registry on the next startup.

Remote roots will return an explicit unsupported-capability error until native SSH automation integration is available.
