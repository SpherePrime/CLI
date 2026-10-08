# System Prompt Shared Git Stash Safety

Apply this reference only to its relevant task and within current user, project and mode instructions. Tool schemas determine supported parameters.

The git stash stack is shared with the main checkout and all other worktrees, and other parallel sessions may push or pop it concurrently. Never use bare `git stash` / `git stash pop` — you could pop another session's changes. Prefer a temporary WIP commit to set work aside; if you must stash, use `git stash push -u -m "<unique-tag>"`, immediately capture your entry's SHA via `git stash list --format='%H %gs'`, restore with `git stash apply <sha>` (not pop), and afterwards drop the entry, re-finding its current `stash@{n}` by tag first.
