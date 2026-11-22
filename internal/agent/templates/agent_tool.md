Launch a specialized subagent for a well-scoped job. Each subagent runs with its own isolated context and cannot see this conversation.

# When to launch subagents

- Broad or open-ended exploration across multiple directories or unknown codebases where searching inline would consume excessive context (`agent="explore"`).
- Independent, self-contained tasks or parallel workstreams that benefit from isolated context or concurrent execution (`agent="code"`, or omit `agent`).
- Dedicated read-only research (`agent="task"`), architecture planning (`agent="plan"`), or optional independent review (`agent="review"`).

# When NOT to launch subagents

- Do NOT launch subagents for simple, narrow, or single-file tasks you can finish inline in a couple of direct tool calls (reading a known file, grepping for a specific symbol, making a small localized edit).
- Trust but verify: do not duplicate searches a subagent is already performing or has returned, but verify its claims, changes, and test results directly before reporting completion.

# Writing subagent prompts

Because subagents cannot see the parent conversation, write a complete, self-contained prompt:
1. Never delegate understanding: synthesize findings yourself and provide exact file paths, line numbers, symbols, and concrete specifications rather than vague phrasing like "based on your findings, fix it".
2. State the exact goal and concrete deliverable.
3. Provide the relevant file paths, symbols, interfaces, and repository conventions (or pass `context_from` / `context_files` to preload explored files).
4. Specify constraints and the user language when user-facing text, UI strings, or docs are involved.
5. For `code` workers, expect them to implement and verify their changes with tests, builds, linters, or runtime checks before reporting back.

# Parameters and usage notes

- `name`: short descriptive name so you and peers can identify or follow up with the worker later.
- `agent`: omit to use `code` (the default read/write/exec worker), or choose `explore`, `task`, `plan`, or `review`.
- `background`: set `true` to run concurrently (launch multiple independent agents in a single turn). Results arrive automatically as team notices when the turn ends — do not poll.
- `wait_for_message`: registers the agent dormant until an `agent_message` or automatic review handoff arrives (implies `background`).
- `context_from` / `context_files`: preload read/edited file context from an earlier worker (for example, `explore` -> `code`) without re-reading unchanged files.
- `reviewer`: optional review agent name to link automatic handoff on completion.
- Follow-up changes: reuse an idle/finished worker via `agent_message(op="send", worker_id=...)` rather than spawning a duplicate worker for the same area.
