# Skill Code Review Conventions Dimension

Apply this reference only to its relevant task and within current user, project and mode instructions. Tool schemas determine supported parameters.

### Conventions (AGENTS.md)

Find the AGENTS.md files that govern the changed code: the user-level
the configured user instruction file, the repo-root AGENTS.md, plus any AGENTS.md or
local project instructions in a directory that is an ancestor of a changed file (a
directory's AGENTS.md only applies to files at or below it). Read each one
that exists, then check the diff for clear violations of the rules they state.

Only flag a violation when you can quote the exact rule and the exact line
that breaks it — no style preferences, no vague "spirit of the doc"
inferences. In the finding, name the AGENTS.md path and quote the rule so the
report can cite it. If no AGENTS.md applies, return nothing for this angle.
