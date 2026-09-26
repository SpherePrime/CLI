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
- **Voice input:** dictate prompts with Whisper on a hotkey, with a live
  microphone indicator
- **Cross-platform:** macOS, Linux, Windows, and the BSDs

## Install

Linux / macOS:

```bash
curl -sSfL https://raw.githubusercontent.com/SpherePrime/CLI/main/scripts/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/SpherePrime/CLI/main/scripts/install.ps1 | iex
```

Prebuilt binaries (`.tar.gz`, `.zip`, `.deb`, `.rpm`, `.apk`):
[releases](https://github.com/SpherePrime/CLI/releases).

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

## Voice input

Press <kbd>alt+v</kbd> (or <kbd>ctrl+shift+space</kbd>) to start recording, and
the same key to stop. The status bar shows `● REC` with a timer while the
microphone is open, and the transcript lands in the prompt at your cursor.

Capture and transcription use external tools, so nothing extra is bundled with
Prime. Dictation runs against a resident local whisper.cpp server: Prime warms
it at TUI launch and stops it about 15 minutes after the last dictation, so the
model is never reloaded per keystroke. Control it with `prime voice server`.
The quickest setup:

```bash
pip install sounddevice                 # microphone capture
prime voice setup                       # whisper.cpp + Large V3 Turbo Q5 model

# or transcribe remotely instead of locally
option voice base-url https://api.openai.com/v1
option voice api-key $OPENAI_API_KEY

option voice language ru                # optional, detected by default
prime voice                             # show what was found
prime voice test                        # record a sample and print the text
```

Other backends, the hotkey setting, and the full priority list:
[docs/voice](docs/voice/README.md).

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
prime voice            # check microphone and Whisper setup
prime voice test       # dictate a sample outside the TUI
prime logs             # print recent logs
```

## Privacy

Prime can record pseudonymous usage metadata (never prompts or responses).
Opt out with `PRIME_DISABLE_METRICS=1`.
