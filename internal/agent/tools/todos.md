Manage a structured task list for multi-step work. Each task has one of three states: pending, in_progress, completed. Keep exactly one task in_progress at a time.

- Create the list up front for any multi-step task; skip it for single-step work.
- Mark a task in_progress before working on it and completed immediately after finishing it.
- After every state change, call this tool again with the full updated list so the UI progress counter (e.g. "2/4") stays accurate.
- Do not leave a task in_progress once its work is done.
- Keep one list for the current task across follow-up requests that add details or continue the same work.
- When the user clearly switches to a different task or direction, replace the old list with a fresh list for the new task. Do not keep completed items from the previous task.
- When the current task is complete and a new task is given, create a new list for it if it has multiple steps; skip the list for a single-step task.
- Use imperative form for content and present-continuous form for active_form (e.g. "Run tests" / "Running tests").
