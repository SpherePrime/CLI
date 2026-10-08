# Agent Prompt Code Review Inline Gap Sweep Phase

Apply this detailed reference to its relevant task; user, project and mode instructions take priority. Example commands require the matching shell and installed utilities.

## Phase 3 — Sweep for gaps

Take one more pass yourself (same context, no subagent) as a fresh reviewer
who has the deduplicated list. Re-read the diff and enclosing functions
looking ONLY for defects not already listed: boundary inputs, platform differences, removed guarantees, error paths, resource cleanup, concurrency and changed caller contracts. Do not repeat already verified findings.
