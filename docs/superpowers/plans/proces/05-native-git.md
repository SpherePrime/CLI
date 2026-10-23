# Native Git and GitHub services

Status: native implementation and isolated repository checks passed. Remote Git parity remains open.

Scope: repository status, history, commit details, diff and GitHub operations through Rust command handlers. Preserve scoped paths, structured arguments, command timeouts and bounded output. Restore/revert require backup, concurrent mutation protection and recovery parity before enabling.

Implemented bounded Git/GitHub command execution, status/history/details/diff, issues/pulls/checks/logs, pull creation and comments. Restore and revert preserve staged, unstaged and untracked files in a stash backup, check the expected HEAD, serialize mutations and recover on failure.

Checks: six isolated repository tests passed in the combined Windows run, including restore backup, reverting one commit while preserving later commits, ignored-file collisions and recovery after a conflict. Fixtures disable automatic CRLF conversion so assertions match actual committed data. No restore/revert was executed against the working project repository. Live GitHub publication and remote SSH Git are not validated by these tests.
