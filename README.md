# Prime

An agentic coding assistant for the terminal. Prime wires your tools,
your code, and your LLM provider of choice into one workflow.

## Features

- **Multi-model:** use any major API provider, or add your own via
  OpenAI- and Anthropic-compatible endpoints, including local servers
- **Flexible:** switch models mid-session without losing context
- **Session-based:** keep multiple work sessions and contexts per project
- **LSP-enhanced:** pulls context from language servers, like a human would
- **Extensible:** add capabilities via MCP servers and agent skills
- **Safe by default:** per-tool permission prompts, with allow/deny lists
- **Plugins:** optional features Prime installs on demand — voice dictation
  with alt+v, downloaded automatically from the plugins menu
- **Cross-platform:** Linux and Windows

## Appearance

Open the command palette with `Ctrl+P` and choose **Interface design** or
**Color theme**. Twelve designs change the layout, input panel, message frames
and colors. Eighteen palettes can be selected independently. Settings persist
across restarts and updates. **Automatic** follows the selected provider.

Shell configuration also supports `option ui design cards` and
`option ui theme nord`.

## Install

Linux / macOS:

```bash
curl -sSfL https://raw.githubusercontent.com/SpherePrime/CLI/main/scripts/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/SpherePrime/CLI/main/scripts/install.ps1 | iex
```

Prebuilt binaries (`.tar.gz`, `.zip`, `.deb`, `.rpm`, `.apk`) for Linux and
Windows:
[releases](https://github.com/SpherePrime/CLI/releases).

> [!NOTE]
> Release artifacts are currently built for Linux and Windows only. The code
> itself compiles for macOS and the BSDs, and `prime update` plus the install
> scripts already know their asset names, but no such artifacts are published
> yet. Build from source on those platforms for now.

With Go installed:

```bash
go install github.com/SpherePrime/CLI@latest
```

Build from source (requires Go):

```bash
go build -o prime .
```

Releases are cut by pushing a version tag:

```bash
git tag v0.1.0 && git push origin v0.1.0
```

### Shell tools on Windows

The shell runs through a bundled Bash-compatible interpreter, so commands and
scripting behave the same everywhere. On Windows a handful of coreutils are
also provided in-process — `cat`, `chmod`, `cp`, `find`, `ls`, `mkdir`, `mv`,
`rm`, `touch`, `xargs`, `base64`, `gzip`/`gunzip`/`gzcat`, `mktemp`, `shasum`
and `tar`.

Utilities outside that set (`sleep`, `head`, `tail`, `wc`, `grep`, `sed`,
`awk`, `sort`, `uniq`, `seq`, `date`, …) are only present if you install them,
for example by putting Git Bash or MSYS2 on `PATH`. Installing them is
recommended; the agent is told which tools exist at runtime either way, and
falls back to its own read/grep/glob tools.

## Getting started

Run `prime` in a project directory:

```bash
prime
```

Press <kbd>ctrl+l</kbd> to open the model picker, choose a provider, and
paste an API key. Providers can also be added from the command palette
(<kbd>ctrl+p</kbd> → Add Provider) or from the CLI:

```bash
prime provider add
```

Many providers are picked up automatically from environment variables,
for example `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, and `GEMINI_API_KEY`.

Local models work too — Prime auto-discovers what the server exposes:

```bash
provider add ollama --type ollama --base-url "http://localhost:11434/v1"
```

## Plugins

Prime has its own optional-features system, separate from MCP servers.
Nothing is installed by default: open the commands menu (`ctrl+p`), run
**plugins**, and toggle a plugin on. Enabling `voice` downloads the Whisper
engine and model on first use (with an in-menu progress bar; if the pieces
are already there it switches on instantly) and binds <kbd>alt+v</kbd> to
dictate into the prompt — recording stops by itself when you go quiet. No
restart is needed to start using it.

| Plugin | What it adds |
| --- | --- |
| `voice` | Microphone dictation with alt+v, transcribed by Google Web Speech |

Voice engine options (`option voice engine|model|language|base-url|api-key ...`)
configure what the plugin runs. State lives in `plugins.json`. See
[docs/plugins](docs/plugins/README.md).

## Configuration

Prime runs great with zero config. When you do want to customize it, the
config format is `primerc` — plain Bash with Prime-specific builtins:

```bash
# Register a provider and a model.
provider add deepseek --type openai-compat \
  --base-url "https://api.deepseek.com/v1" \
  --api-key "$DEEPSEEK_API_KEY"
model add deepseek/deepseek-chat --name "Deepseek V3" --context-window 64000

# Auto-approve some tools.
permissions allow view edit

# Add an MCP server and an LSP.
mcp add filesystem --command node --args /path/to/mcp-server.js
lsp add go --command gopls

# Misc options.
option notifications disabled
```

Config files are searched in this order (first match wins):

| Priority | Unix-like                 | Windows                               |
| -------- | ------------------------- | ------------------------------------- |
| 1        | `./.primerc`              | `.\.primerc`                          |
| 2        | `./primerc`               | `.\primerc`                           |
| 3        | `~/.config/prime/primerc` | `%USERPROFILE%\.config\prime\primerc` |

A JSON format (`prime.json`) is still supported but deprecated.

> [!WARNING]
> `primerc` and `prime.json` are trusted code: `primerc` runs in a real
> shell and command substitution executes at load time. Only keep configs
> from sources you trust.

## Context files

Prime reads project instructions from context files such as `AGENTS.md`
and personal, cross-project instructions from `~/.config/prime/PRIME.md`.
Use a `.primeignore` file (gitignore syntax) to keep files out of context.

## CLI

```bash
prime                  # start the TUI
prime run "prompt"     # one-shot agent run
prime provider add     # add a provider interactively
prime models           # list known models
prime sessions         # manage sessions
prime stats            # token and cost stats
prime logs             # print recent logs
```

## Privacy

Prime can record pseudonymous usage metadata (never prompts or responses).
Opt out with `PRIME_DISABLE_METRICS=1`.
