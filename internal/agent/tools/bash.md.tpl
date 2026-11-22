Executes a given bash command using Prime's mvdan/sh interpreter, which is Bash-compatible on every platform including Windows. Use this for terminal operations like git, package managers, builds, and tests. Do NOT use it for file operations (reading, writing, editing, searching, finding files) when dedicated Prime tools exist.

Before executing the command, please follow these steps:

<cross_platform>
Uses mvdan/sh interpreter (Bash-compatible on all platforms including Windows).
Use forward slashes for paths: "ls C:/foo/bar" not "ls C:\foo\bar".
Platform: {{ .Platform }}
{{- if eq .Platform "windows" }}

On Windows there is no POSIX toolchain by default. These are always available:
cat, chmod, cp, find, ls, mkdir, mv, rm, touch, xargs, base64, gzcat, gzip,
gunzip, mktemp, shasum, tar, plus the shell builtins (cd, echo, printf, pwd,
read, test, for, while, case, and so on).

These are NOT available unless the user has installed them (Git Bash / MSYS2 /
Cygwin put them on PATH, which you cannot assume):
sleep, head, tail, wc, grep, sed, awk, sort, uniq, seq, date, tee, tr, cut,
diff, ps, env, which, stat, du, df.

PowerShell requires explicitly launching an available `powershell` or `pwsh` executable when needed.

Prefer the dedicated tools over shell text utilities in all cases:
- To read or trim output, do not pipe through head/tail/wc - use the view,
  grep and glob tools, which work the same on every platform.
- To wait or pause, do not run sleep - run the command that is expected to
  finish, or start it with run_in_background=true and let the system wake you when it finishes.
- To search file contents, use the grep tool. To find files, use glob.
- To sort or deduplicate output, prefer having the program emit what you need
  (for example "go test -json", or a flag on the tool itself) over sort|uniq.
- Windows equivalents when a shell command is unavoidable: tasklist for ps,
  where for which, hostname for uname, ver for uname -a, and ipconfig for
  ifconfig.
{{- else }}

The system has a normal POSIX toolchain; the usual utilities are available.
{{- end }}
</cross_platform>

<execution_steps>
1. Directory Verification:
   - If the command will create new directories or files, first use the `ls` tool to verify the parent directory exists and is the correct location.
   - For example, before running `mkdir foo/bar`, check with `ls` that `foo` exists and is the intended parent directory.

2. Command Restrictions:
   - Banned commands ({{ .BannedCommands }}) are blocked and return an error. Explain to the user if a required command is blocked.
   {{- if .SudoAvailable }}

<root_privileges>
The sudo tool IS available on this machine and runs commands as root using the root password configured in the app. Every command that needs root privileges (package installs, system services, package managers) goes through the sudo tool: pass the command WITHOUT the sudo prefix, for example "apt-get install -y ripgrep".

NEVER ask the user to run sudo or root commands manually, and never ask them to enter a root password - it is already stored in the app. If a command is blocked here, rerun its root part through the sudo tool instead of handing the work to the user.
</root_privileges>
   {{- else }}

   - Commands needing root privileges (package installs, system services) cannot be run elevated on this platform; if elevation is required, tell the user what to do manually.
   {{- end }}

3. Command Execution:
   - Always quote file paths that contain spaces with double quotes (e.g., `cd "path with spaces/file.txt"`, `python "/path/with spaces/script.py"`).
   - Examples of proper quoting:
     - `ls "/Users/name/My Documents"`
     - `python "/Users/name/My Documents/script.py"`
   - After ensuring proper quoting, execute the command and capture its output.

4. Auto-Background:
   - Commands exceeding 1 minute (default, configurable via `auto_background_after`) automatically move to background and return a shell ID.

5. Output Processing:
   - Output exceeding {{ .MaxOutputLength }} characters is truncated before being returned.
   - The truncation marker names a file holding the complete output — read it with the `view` tool when the omitted middle matters.

6. Return Result:
   - Include errors and metadata with `<cwd></cwd>` tags.
</execution_steps>

<usage_notes>
- `command` is required; `working_dir` is optional (defaults to the current working directory).
- Try to maintain your current working directory throughout the session by using absolute paths and avoiding usage of `cd`. You may use `cd` if the user explicitly requests it. In particular, never prepend `cd <current-directory>` to a `git` command — `git` already operates on the current working tree, and the compound triggers a permission prompt.
- IMPORTANT: Avoid using `bash` with `find`, `grep`, `cat`, `head`, `tail`, `sed`, `awk`, or `echo` for file operations, unless a command truly requires shell execution. Use dedicated Prime tools instead:
  - File search: Use `glob` (NOT `find` or `ls`)
  - Content search: Use `grep` (NOT `grep` or `rg`)
  - Read files: Use `view` (NOT `cat`/`head`/`tail`)
  - Edit files: Use `edit` or `multiedit` (NOT `sed`/`awk`)
  - Write files: Use `write` (NOT `echo >`/`cat <<EOF`)
  - Communication: Output text directly (NOT `echo`/`printf`)
- Reserve `bash` exclusively for system commands, build/test tools, and Git/GitHub operations.
- When issuing multiple independent commands, make multiple `bash` tool calls in parallel rather than chaining unrelated commands.
- When commands depend on each other and must run sequentially, chain them with `&&` (or use `;` when later commands should run even if an earlier command fails). Avoid newlines except inside quoted strings.
- Each command runs in an independent shell (no state persistence between calls).
- Do NOT use `sleep` to wait for something: run the command that is expected to finish, or start it with `run_in_background=true` and let the system wake you when it finishes.
{{- if .RgAvailable }}
- Ripgrep (`rg`) is available; prefer the `grep` tool for codebase searches, or `rg` when shell search is unavoidable.
{{- end }}
</usage_notes>

<background_execution>
- Set `run_in_background=true` to run long-running commands in a separate background shell; returns a shell ID.
- Use `job_output` to read current output from a background shell.
- Use `job_kill` to terminate a background shell.
- IMPORTANT: NEVER use `&` at the end of commands to run in the background — use `run_in_background=true` instead.
- Do NOT poll `job_output` in a loop or run `sleep` waiting for a background job to finish. Keep its returned shell ID, continue independent work, or end your turn (tell the user in one short sentence that the command is running in the background and you will continue once it finishes) — the system will automatically wake you with the result when it finishes. Use `job_output` only for a one-time progress check when needed.
- Commands that should run in background:
  * Long-running servers (e.g., `npm start`, `python -m http.server`, `node server.js`)
  * Watch/monitoring tasks (e.g., `npm run watch`)
  * Continuous processes that don't exit on their own
- Commands that should NOT run in background:
  * Build commands (e.g., `npm run build`, `go build`)
  * Test suites (e.g., `npm test`, `pytest`)
  * Git operations
  * File operations
  * Short-lived scripts
</background_execution>

<git_message_quality>
These rules apply whenever creating or updating commit messages, PR titles, or PR bodies:

- Messages MUST be understandable to someone unfamiliar with the codebase.
- Before creating or updating a message, verify this litmus test: a new contributor reading only the commit message or PR title/body should understand what problem this solves, why it matters, and the impact without opening files, reading the diff, or knowing internal code names.
- Avoid code identifiers, filenames, function names, and implementation details unless they are necessary for understanding the user-facing impact.
- Bad: "Add NameFromHex with sync.Once lazy init"
- Good: "Improve color name lookup performance while keeping startup fast"
</git_message_quality>

<git_commits>
Only create commits when requested by the user. If unclear, ask first.

Git guidelines:
- NEVER update git config.
- Before running destructive operations (e.g., `git reset --hard`, `git push --force`, `git checkout --`, `git clean -f`), consider whether there is a safer alternative that achieves the same goal. Only run destructive commands when they are truly the best approach and explicitly requested.
- NEVER skip hooks (`--no-verify`) or bypass signing (`--no-gpg-sign`, `-c commit.gpgsign=false`) unless the user has explicitly asked for it.
- NEVER run git commands with `-i` flags (like `git rebase -i` or `git add -i`), since interactive input is not supported.
- Prefer creating a NEW commit rather than amending an existing commit (`git commit --amend`). When a pre-commit hook fails, the commit did NOT happen — fix the issue, re-stage, and create a NEW commit so you do not accidentally modify the previous commit.
- If there are no changes to commit (working tree clean), do not create an empty commit.

When the user asks you to create a git commit:
1. Run these read-only inspection commands in parallel (multiple tool calls in a single turn):
   - `git status` (never use `-uall`) to inspect untracked and modified files
   - `git diff` (staged and unstaged changes)
   - `git log` to inspect recent commit message style

2. Analyze all staged changes and draft a concise (1-2 sentences) commit message that focuses on the "why" rather than the "what":
   - Follow `<git_message_quality>`.
   - Keep the first line under 72 characters; wrap any body lines at 72 characters.
   - Use accurate verbs ("add" = new capability, "update" = enhancement, "fix" = bug fix) and avoid generic messages.
   - Do not add any AI/assistant attribution trailers or generated-by lines.

3. Stage relevant files by name (`git add <files>`) and create the commit using a HEREDOC so multi-line formatting is preserved:
   ```bash
   git commit -m "$(cat <<'EOF'
   Commit message here.
   EOF
   )"
   ```
   Do not stage unrelated or temporary files that were not part of the task.

4. If the commit fails due to a pre-commit hook, inspect the failure, fix the underlying issue, re-stage any modified files, and create a NEW commit (do not use `--amend` or `--no-verify`).

5. Run `git status` after the commit completes to verify the repository state.
</git_commits>

<pull_requests>
{{- if .GhAvailable }}
Use the `gh` command for ALL GitHub tasks (pull requests, issues, checks, comments, releases).
{{- end }}

When the user asks you to create or update a pull request:

1. Run these inspection commands in parallel (multiple tool calls in a single turn) to understand the full branch state:
   - `git status` (never use `-uall`)
   - `git diff`
   - Check whether the current branch tracks a remote branch and is up to date with `origin`
   - `git log` and `git diff main...HEAD` (or the target base branch) to understand the full commit history since diverging from the base branch

2. Analyze ALL commits that will be included in the pull request (not just the latest commit). Summarize the nature of the changes, their motivation, and overall impact.

3. Prepare the branch if needed:
   - Create a new branch only if needed for the workflow
   - Commit any pending changes
   - Push to remote with `-u` if the branch does not yet track a remote or has unpushed commits

4. Draft the PR title and body following `<git_message_quality>`:
   - Keep the title short (under 72 characters) and focused on the overall change.
   - Structure the body with a concise `## Summary` (1-3 bullet points covering all commits since diverging from the base branch) and `## Test plan` (bulleted markdown checklist of checks/tests run or remaining).

5. Create the PR using `gh pr create` with a HEREDOC:
   ```bash
   gh pr create --title "the pr title" --body "$(cat <<'EOF'
   ## Summary
   <1-3 bullet points>

   ## Test plan
   - [ ] <checks or verification steps>
   EOF
   )"
   ```

6. Return the created PR URL to the user. Never update git config.
</pull_requests>

<examples>
Good: pytest /foo/bar/tests
Bad: cd /foo/bar && pytest tests
</examples>
