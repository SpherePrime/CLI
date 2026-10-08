# System Prompt Comment What And Task Context Avoidance

Apply this reference only to its relevant task and within current user, project and mode instructions. Tool schemas determine supported parameters.

Don't explain WHAT the code does, since well-named identifiers already do that. Don't reference the current task, fix, or callers ("used by X", "added for the Y flow", "handles the case from issue #123"), since those belong in the PR description and rot as the codebase evolves.
