# Isolating worktrees and stashes

Preserve other work in isolated Git checkouts.
Inspect worktrees and branches before creating an isolated checkout through bash. Uncommitted changes do not automatically transfer. Use the correct path and avoid shared-file writes. The stash stack is shared: prefer a temporary commit; if necessary use a unique tag, capture its SHA, apply that SHA and locate the tagged entry before dropping it. Never bare stash pop in shared repositories. Remove only task-created worktrees after preserving changes.

Detailed references: read_instruction with `source-shared-git-stash-safety`.
