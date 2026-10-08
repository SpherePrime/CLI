# Skill Run Skill Template

Apply this detailed reference to its relevant task; user, project and mode instructions take priority. Example commands require the matching shell and installed utilities.

This is an authoring template, not executable instructions. Replace all angle-bracket slots with verified project values before saving a reusable run skill. Do not execute placeholder commands or install illustrative dependencies.

---
name: run-<unit-name>
description: Build, run, and drive <unit-name>. Use when asked to start <unit-name>, run its tests, build it, take a screenshot of its UI, or interact with the running app.
---

<One-sentence description: what this is and how an agent drives it.
Name the handle here - "drive it via
the configured local skill directory and a compatible driver" for a desktop
app, or "start the dev server then drive it via the available browser driver" for a
web app - so an agent knows where to look first.>

<If the unit isn't at repo root:>
All paths below are relative to `<unit-dir>/`.

## Prerequisites

<System-level requirements. The exact verified setup command for the current operating system and shell. Do not assume Ubuntu or a particular package manager.>

```bash
<verified-platform-setup-command-if-needed>
```

<Runtime versions if they matter:>

```bash
# Example: Node 20 via nvm, Python 3.12 via uv, etc.
```

## Setup

<One-time setup after clone: install deps, configure, apply any
patches (feature-gate overrides, config stubs) with the exact command.>

```bash
<commands>
```

<Env vars - required vs optional, with sensible defaults:>

```bash
<set-required-and-optional-environment-variables-with-current-shell-syntax>
<document-credential-source-without-recording-secret-values>
```

## Build

<Skip if no separate build step. Otherwise the exact command:>

```bash
<command>
```

## Run (agent path)

<This is the section a future agent actually uses. If you built a
driver/REPL/smoke script, this documents how to launch it and what it
does. If a permitted HTTP client or a one-liner suffices,
that one-liner goes here.>

```bash
<launch-the-driver-or-smoke-script>
```

<For REPL-style drivers, use the following tmux pattern only if tmux and Bash are already available; otherwise document the actual supported driver. Poll for a ready marker
between send-keys and capture-pane - faster than a fixed sleep and fails
loudly instead of capturing a half-rendered screen:>

```bash
tmux new-session -d -s app -x 200 -y 50
tmux send-keys -t app '<launch command>' Enter
timeout 30 bash -c 'until tmux capture-pane -t app -p | grep -q "<ready-marker>"; do sleep 0.2; done'
tmux send-keys -t app '<first driver command>' Enter
tmux capture-pane -t app -p
```

<Where artifacts land (screenshots, logs) - absolute paths:>

Screenshots -> `<verified-absolute-screenshot-directory>`. Logs -> `<verified-absolute-log-path>`.

<If the driver has commands, a table:>

| command | what it does |
|---|---|
| `<cmd>` | <description> |

## Run (human path)

<If meaningfully different from the agent path. Brief - agents won't
use this, humans can figure it out.>

```bash
<command>   # -> <what happens>. <how to stop>.
```

## Test

```bash
<command>
```

<Expected result - "N suites pass", or specific known-flaky tests.>

---

<Optional sections below - include only if relevant and only with
content you actually hit, not generic advice.>

## Gotchas

<Non-obvious traps. The things that look like they should work but
don't, with the workaround. If this section is generic, delete it.>

- **<specific thing>** - <why it breaks> -> <what to do instead>

## Troubleshooting

<Symptom -> fix. Only errors you actually encountered.>

- **<exact error message or symptom>**: <cause>. <fix>.

<---

NOTE ON THE FRONTMATTER ABOVE:
- Replace <unit-name> in both `name:` and `description:`. Use a valid skill name matching the local skill loader requirements; do not assume automatic slash-command registration.
- The `description:` is what the skill loader scans to decide whether to load this
  skill automatically. Keep the verbs - "start," "run," "build," "test,"
  "screenshot" - they're what an asking agent will actually type.

NOTE ON THE DRIVER:
- If you wrote a driver script, it lives in this same directory (next
  to this file) by default. Reference it from the Run section.
- For a web app there's usually no driver file - the supported browser automation
  invocation in the Run section is the harness.
- If the driver grows into something the project's test suite wants -
  shared launch helpers, a real e2e harness - move it to scripts/ or
  e2e/ in the unit, and update the paths here. The skill stays put.

Delete everything from `---` above onwards before committing. --->
