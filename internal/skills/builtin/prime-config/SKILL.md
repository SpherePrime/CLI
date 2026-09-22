---
name: prime-config
description: Use when the user needs help configuring Prime — writing primerc (the Bash config format) or the per-section JSON files (providers.json, models.json, mcp.json, lsp.json, skills.json, options.json), setting up providers, models, LSPs, MCP servers, hooks, skills, permissions, or changing Prime behavior.
---

# Prime Configuration

Prime supports two config formats:

- **`primerc`** — a Bash script that builds config by calling Prime builtins.
  **Preferred.** Because it is real Bash you get includes, secrets,
  conditionals, and variables for free.
- **JSON** — static files, one per section, in the config directory. Fully
  supported; see [JSON storage layout](#json-storage-layout).

Both are discovered together and deep-merged. Priority (highest to lowest):

1. `.primerc` / `primerc` / `.prime.json` / `prime.json` (project-local,
   closer-to-cwd wins; Windows uses `.\.primerc` / `.\primerc`)
2. The project data directory (`.prime/` by default): `options.json`,
   `providers.json`, and the other section files, plus `prime.json`
3. `$XDG_CONFIG_HOME/prime/primerc` or `~/.config/prime/primerc`
   (`%XDG_CONFIG_HOME%\prime\primerc` or
   `%USERPROFILE%\.config\prime\primerc` on Windows)
4. The global section files in the same config directory

Within one directory the section files win over that directory's `prime.json`,
and `primerc` wins over JSON.

Data directories (`~/.local/share/prime` and `%LOCALAPPDATA%\prime`) contain
machine-owned state only: caches, provider catalogs, `projects.json`, logs, and
locks. Older Prime releases kept the global config there as a single
`prime.json`; on the next start Prime moves its keys into the config directory
section files and leaves the original as `prime.json.bak`. Prime does not
discover or execute a `primerc` from those locations.

## primerc at a glance

A `primerc` is a plain Bash script executed at load time with the same embedded
shell the `bash` tool uses. It builds config by calling builtins (`provider`,
`model`, `mcp`, `lsp`, `hook`, `permissions`, `option`). Statements run top to
bottom; later statements win, and `remove`/`reset` operate on anything defined
earlier or pulled in via `source`.

```bash
#!/usr/bin/env bash
# Includes and secrets are just Bash.
source ~/.config/prime/shared.sh

provider add anthropic --api-key "$ANTHROPIC_API_KEY"

model large anthropic/claude-sonnet-4-20250514 --max-tokens 16384
model small anthropic/claude-haiku-4-20250514

option skill-path ./skills
permissions allow view ls grep edit
```

Values are ordinary Bash — quote and expand normally (`"$VAR"`, `$(cmd)`,
`${VAR:?required}`). A failing `$(command)` aborts the load.

`PRIME_VERSION` is exported into the script so you can feature-detect the
running Prime (it is the literal `devel` for local builds):

```bash
[[ "$PRIME_VERSION" != devel ]] && lsp add gopls --command gopls
```

## Commands

All entity commands are verb-first. `remove` accepts `rm` as an alias. Booleans
accept `true/false/1/0/yes/no`, case-insensitive.

### providers

```bash
provider add <id> [flags]    # define/update; repeated calls merge
provider remove <id>         # alias: rm — removes the provider and its models
```

Flags: `--name`, `--type` (`openai`, `openai-compat`, `anthropic`, or a local
type like `ollama`, `lmstudio`, `llamacpp`), `--api-key`, `--base-url`,
`--disable BOOL`, `--flat-rate BOOL`, `--discover-models BOOL`,
`--system-prompt-prefix TEXT`, `--extra-header KEY VALUE` (repeatable),
`--extra-body JSON`, `--provider-options JSON`.

```bash
provider add deepseek \
  --type openai-compat \
  --base-url "https://api.deepseek.com/v1" \
  --api-key "${DEEPSEEK_API_KEY:?set DEEPSEEK_API_KEY}"
```

### models

```bash
model add <provider>/<id> [flags]      # register a custom model (provider must exist)
model remove <provider>/<id>           # alias: rm
model large [<provider>/<id>] [flags]  # set the large slot; no arg prints it
model small [<provider>/<id>] [flags]  # set the small slot; no arg prints it
```

- `<provider>/<id>` is the same form `prime models` prints. A missing slash is
  an error. `model add` requires the provider to already exist.
- `model add` flags: `--name`, `--context-window N`, `--default-max-tokens N`,
  `--can-reason BOOL`, `--supports-images BOOL`, `--price-input F`,
  `--price-output F`, `--price-cache-create F`, `--price-cache-hit F`,
  `--reasoning-effort low|medium|high`.
- `model large`/`model small` flags: `--think`, `--reasoning-effort`,
  `--max-tokens N`, `--temperature F`, `--top-p F`, `--top-k N`,
  `--frequency-penalty F`, `--presence-penalty F`, `--provider-options JSON`.
- `model large` with no argument prints the current selection as `provider/id`,
  usable in `$(model large)`.

`large` is the primary coding model; `small` is used for summarization.

### mcp

```bash
mcp add <name> --type stdio|sse|http [flags]   # default type is stdio
mcp remove <name>                              # alias: rm
```

Flags: `--command CMD`, `--args ARG` (repeatable), `--env KEY VALUE`
(repeatable), `--url URL`, `--header KEY VALUE` (repeatable), `--timeout N`,
`--disabled BOOL`, `--disabled-tools TOOL` (repeatable), `--enabled-tools TOOL`
(repeatable), `--oauth BOOL`, `--oauth-client-id ID`, `--oauth-client-secret SECRET`,
`--oauth-callback-port PORT`.

```bash
mcp add github --type http \
  --url "https://api.githubcopilot.com/mcp/" \
  --header Authorization "Bearer $GH_PAT"

mcp add filesystem --command node --args /path/to/mcp-server.js
```

### lsp

```bash
lsp add <name> --command CMD [flags]
lsp remove <name>                     # alias: rm
```

Flags: `--args ARG` (repeatable), `--env KEY VALUE` (repeatable),
`--filetypes TYPE` (repeatable), `--root-markers MARKER` (repeatable),
`--timeout N`, `--disabled BOOL`, `--init-options JSON`, `--options JSON`.

```bash
lsp add go --command gopls --env GOPATH "$HOME/go"
lsp add typescript --command typescript-language-server --args --stdio
```

### hooks

```bash
hook add <event> --command CMD [--name NAME] [--matcher REGEX] [--timeout N]
hook remove <event> [--name NAME]    # alias: rm; without --name clears the event
```

Only named hooks can be removed individually — give a hook `--name` if you
intend to remove it later. See [Hooks runtime](#hooks-runtime) for how hooks
execute (stdin payload, env vars, decisions).

```bash
hook add PreToolUse --matcher "^bash$" --command ".prime/hooks/no-haskell.sh" --name no-haskell
```

### permissions

```bash
permissions allow <tool> [<tool> ...]   # tools that skip permission prompts
permissions deny <tool> [<tool> ...]    # hide tools from the agent entirely
```

`deny` is the inverse of `allow`: it writes `options.disabled_tools`. A denied
tool is hidden from the agent, not merely prompted for.

### options

```bash
option <key> [value]
option reset <list-key>    # clear a list option back to empty
```

- **Boolean keys** (value optional, defaults `true`): `debug`, `debug-lsp`,
  `auto-lsp`, `progress`.
- **Boolean keys phrased positively** (stored as the negated field): `metrics`,
  `auto-summarize`, `provider-auto-update`,
  `default-providers`. Example: `option metrics false` disables metrics.
- **String keys**: `data-directory`, `initialize-as`, `notifications`.
- **Attribution keys**: `attribution-trailer-style` (`none`, `co-authored-by`,
  `assisted-by`) and `attribution-generated-with` (boolean).
- **UI settings**: `option ui compact BOOL`, `option ui diff unified|split`,
  `option ui transparent BOOL`, `option ui mouse BOOL` (default `true`;
  disable to let the terminal/tmux handle selection, copy/paste, and
  scrolling), `option ui scrollbar default|always|never`,
  `option ui exit-banner default|compact|none`,
  `option ui completions-max-depth N`, `option ui completions-max-items N`.
- **List keys** (singular, one value per call, repeatable): `context-path`,
  `global-context-path`, `skill-path`, `disable-skill`. Use `option reset <key>`
  to wipe inherited values (e.g. after `source`).

```bash
option progress false
option skill-path ./skills
option disable-skill prime-config
option attribution-trailer-style assisted-by
option attribution-generated-with true
option ui compact true
option ui diff unified
option ui exit-banner compact
```

> [!IMPORTANT] These skill paths are loaded by default and do NOT need
> `skill-path`: `.agents/skills`, `.prime/skills`, `.claude/skills`,
> `.cursor/skills`.

## Installing third-party skills

Prime has no plugin manager and no plugin API: a skill is a folder with a
`SKILL.md`, and installing one means making that folder discoverable. A README
that says "install as a plugin for OpenCode / Claude Code / Cursor / Codex", or
that writes to `~/.config/opencode/`, describes a different product. Do that
only when the user names it. For Prime:

```bash
# 1. Clone into the global skills directory. It is searched recursively for
#    <anything>/<skill-name>/SKILL.md, so no config change is needed.
git clone https://github.com/obra/superpowers ~/.config/prime/skills/superpowers

# 2. Only if the pack needs always-on bootstrap text in every session, register
#    one of its files as global context.
echo 'option global-context-path "$HOME/.config/prime/skills/superpowers/skills/using-superpowers/SKILL.md"' \
  >> ~/.config/prime/primerc
```

Keeping the clone elsewhere (a vendored folder in a repo, a shared checkout)
needs the directory registered instead:

```bash
option skill-path "$HOME/tools/superpowers/skills"
```

which is the same as `skills_paths` in JSON. Because skill folders are scanned
recursively, register the directory that *contains* the per-skill folders.

```json
// ~/.config/prime/skills.json
{
  "options": {
    "skills_paths": ["~/.config/prime/skills/superpowers/skills"]
  }
}
```

Skills and context files load at startup, so a running session won't see them
until Prime restarts. Check the result with `prime dirs` (config directory) and
by asking for one of the new skill names.

## Hooks runtime

Hooks are user-defined shell commands that fire on agent events. Currently only
`PreToolUse` is supported, which runs before a tool executes. This behavior is
the same however the hook is defined (`hook add` or JSON).

### How hooks work

1. When a tool is about to be called, all `PreToolUse` hooks with a matching
   `matcher` (or no matcher) run in parallel.
2. Duplicate commands are deduplicated — each unique command runs at most once.
3. The hook receives JSON on **stdin** and hook-specific **environment
   variables**.

Event names are case-insensitive and accept snake_case: `PreToolUse`,
`pretooluse`, `pre_tool_use`, `PRE_TOOL_USE` all work.

### Hook input (stdin)

```json
{
  "event": "PreToolUse",
  "session_id": "abc-123",
  "cwd": "/path/to/project",
  "tool_name": "bash",
  "tool_input": { "command": "ls -la" }
}
```

### Hook environment variables

| Variable                     | Description                                       |
| ---------------------------- | ------------------------------------------------- |
| `PRIME_EVENT`                | Event name (e.g. `PreToolUse`)                    |
| `PRIME_TOOL_NAME`            | Name of the tool being called                     |
| `PRIME_SESSION_ID`           | Current session ID                                |
| `PRIME_CWD`                  | Current working directory                         |
| `PRIME_PROJECT_DIR`          | Project root directory                            |
| `PRIME_TOOL_INPUT_COMMAND`   | Value of `command` from tool input (if present)   |
| `PRIME_TOOL_INPUT_FILE_PATH` | Value of `file_path` from tool input (if present) |

### Hook output

**Exit code 0** — hook succeeded. Stdout is parsed as JSON:

```json
{ "decision": "allow", "context": "optional context appended to tool result" }
```

- `decision`: `allow` to explicitly allow, `deny` to block, `none` (or omit).
- `reason`: explanation (used when denying).
- `context`: extra context appended to the tool result.
- `updated_input`: replacement JSON for the tool input; last non-empty wins.

**Exit code 2** — the tool call is blocked; stderr is the deny reason.

**Any other exit code** — non-blocking error; the tool call proceeds.

### Decision aggregation

- **Deny wins over allow** — any deny blocks the call.
- **Allow wins over none** — a lone allow lets it proceed.
- Deny reasons and context strings are concatenated (newline-separated).
- For `updated_input`, the last non-empty value wins.

### Claude Code compatibility

Prime also accepts the Claude Code hook output format, so existing hooks work
unchanged:

```json
{
  "hookSpecificOutput": {
    "permissionDecision": "allow",
    "permissionDecisionReason": "Auto-approved",
    "updatedInput": { "command": "echo rewritten" }
  }
}
```

## User-invocable skills

Skills can be invoked as commands. Add `user-invocable: true` to the skill's
YAML frontmatter:

```yaml
---
name: my-skill
description: A skill that can be invoked as a command.
user-invocable: true
---
```

- Global skills appear as `user:skill-name`; project skills as
  `project:skill-name`.
- Add `disable-model-invocation: true` to keep a skill user-only (hidden from
  the model's available-skills list but still manually invocable).

## Environment variables

- `PRIME_VERSION` — exported into `primerc` at load; the running version (or
  `devel` for local builds).
- `PRIME_GLOBAL_CONFIG` — override the global config directory (where the
  section JSON files live).
- `PRIME_GLOBAL_DATA` — override data directory location.
- `PRIME_SKILLS_DIR` — override default skills directory.

## JSON storage layout

Persisted settings are JSON files, one per section, in the config directory
(`~/.config/prime`, or `PRIME_GLOBAL_CONFIG`). A project uses the same split in
its data directory (`.prime/` by default). Every file repeats the schema path it
owns, so `providers.json` starts with `"providers"`.

| File             | Keys stored                                                     |
| ---------------- | --------------------------------------------------------------- |
| `providers.json` | `providers`                                                      |
| `models.json`    | `models`, `recent_models`                                        |
| `mcp.json`       | `mcp`                                                            |
| `lsp.json`       | `lsp`                                                            |
| `skills.json`    | `options.skills_paths`, `options.disabled_skills`                 |
| `options.json`   | the rest of `options`, plus `tools`, `permissions`, `hooks`, `env` |
| `prime.json`     | `$schema` and any key with no section of its own                  |

Write to the file that owns the key you are changing; a key in the wrong file
still loads but is overridden by the owning section on the next migration.
Merging is a deep merge of everything found, so a value set in several places
resolves by the priority list above.

```json
// ~/.config/prime/providers.json
{
  "providers": {
    "anthropic": { "api_key": "$ANTHROPIC_API_KEY" }
  }
}
```

`$schema` is optional and only enables IDE autocomplete; put it in
`prime.json`.

### primerc ↔ JSON mapping

| primerc                             | file + key                                               |
| ------------------------------------ | -------------------------------------------------------- |
| `provider add openai --api-key "$K"` | `providers.json`: `providers.openai = {"api_key": "$K"}` |
| `model add openai/gpt-x --name X`    | `providers.json`: append to `providers.openai.models[]`  |
| `model large openai/gpt-x`           | `models.json`: `models.large = {"provider":"openai","model":"gpt-x"}` |
| `mcp add gh --type http --url U`     | `mcp.json`: `mcp.gh = {"type":"http","url":"U"}`         |
| `lsp add go --command gopls`         | `lsp.json`: `lsp.go = {"command":"gopls"}`               |
| `hook add PreToolUse --command C`    | `options.json`: append to `hooks.PreToolUse[]`           |
| `permissions allow view ls`          | `options.json`: `permissions.allowed_tools = ["view","ls"]` |
| `permissions deny bash`              | `options.json`: `options.disabled_tools = ["bash"]`      |
| `option skill-path ./skills`         | `skills.json`: `options.skills_paths = ["./skills"]`     |
| `option disable-skill prime-config`  | `skills.json`: `options.disabled_skills = ["prime-config"]` |
| `option smart-tools true`            | `options.json`: `options.smart_tools = true`             |
| `option metrics false`               | `options.json`: `options.disable_metrics = true`         |
| `option request-timeout 300`          | `options.json`: `options.request_timeout = 300`          |
| `option attribution-trailer-style none` | `options.json`: `options.attribution.trailer_style = "none"` |
| `option attribution-generated-with false` | `options.json`: `options.attribution.generated_with = false` |

### Shell expansion in JSON

In JSON, only selected string fields are run through the embedded shell at load
time (in `primerc`, everything is native Bash so this table does not apply):

| Surface                                                         | Expansion                          |
| --------------------------------------------------------------- | ---------------------------------- |
| Provider `api_key`, `base_url`, `api_endpoint`, `extra_headers` | yes                                |
| Provider `extra_body`                                           | **no** (JSON passthrough)          |
| MCP `command`, `args`, `env`, `headers`, `url`                  | yes                                |
| LSP `command`, `args`, `env`                                    | yes                                |
| Hook `command`                                                  | runs via `sh -c`, not the resolver |

Supported constructs: `$VAR`, `${VAR}`, `${VAR:-default}`, `${VAR:+alt}`,
`${VAR:?message}`, `$(command)`. An unset variable expands to empty; a failing
`$(command)` is a hard error. A header that resolves to empty is dropped from
the request.

### Security note

Both formats are trusted code. `primerc` runs entirely, and any `$(...)` in
`prime.json` runs at load time, with the invoking user's shell privileges,
before the UI appears. Don't launch Prime in a directory whose config you
haven't reviewed.
