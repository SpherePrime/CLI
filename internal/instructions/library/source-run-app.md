# Skill Run App

Apply this detailed reference to its relevant task; user, project and mode instructions take priority. Example commands require the matching shell and installed utilities.

---
name: run
description: Launch and drive this project's app to see a change working. Use when asked to run, start, or screenshot the app, or to confirm a change works in the real app (not just tests). First looks for a project skill that already covers launching the app; otherwise falls back to built-in patterns per project type (CLI, server, TUI, Electron, browser-driven, library).
---

**Running means launching the actual app and interacting with it** -
not the test suite, not an `import` of an internal function and a
`console.log`. The app as a user (human or programmatic) would meet
it: the CLI at its command, the server at its socket, the GUI at its
window.

## First: does a project skill already cover this?

A project skill that launches this app is the repo's verified path -
inspect its setup, environment variables and driver; prefer the established project recipe when it applies to the current host.

Use search_skills to find project launch instructions, then view the returned SKILL.md before following it. Verify the instructions against the current environment.

- **One describes launching/driving this app** -> read that SKILL.md
  and follow applicable steps without assuming its host-specific commands fit every environment.
- **Mega-repo, several plausible, no clear match** -> ask the user
  which unit to run.
- **Stale** (fails on mechanics unrelated to your task) -> tell the
  user; repair the applicable project run instructions if this is in scope.
- **Nothing about running** -> fall back to the patterns below.

## Otherwise: match the shape, use the pattern

Pick the row closest to your project. Each example walks through
launch + first interaction; ignore any trailing "write the skill"
section - you're using the recipe, not authoring one.

| Project type | Handle | Example |
|---|---|---|
| CLI tool | direct invocation, exit code, stdin/stdout | `read_instruction: source-run-cli-tool-example` |
| Web server / API | bash background job + permitted HTTP client | `read_instruction: source-run-web-server-api-example` |
| TUI / interactive terminal | tmux `send-keys` / `capture-pane` | `read_instruction: source-run-tui-interactive-terminal-app-example` |
| Electron / desktop GUI | available desktop automation driver | a discovered compatible desktop automation skill/tool, if available |
| Browser-driven | dev server + available browser driver | a discovered compatible browser skill/tool, if available |
| Library / SDK | import-and-call smoke script at the package boundary | `read_instruction: source-run-library-sdk-example` |

If nothing fits, start from the closest match and adapt. For a web
app, a discovered compatible browser skill/tool, if available - use only the browser driver actually available. For a desktop app,
a discovered compatible desktop automation skill/tool, if available - choose a compatible available driver, or explicitly report unverified GUI interaction.

## Drive it, don't just launch it

Launching with no interaction proves the entrypoint resolves. That's
not running the app - it's typechecking with extra steps. Drive it to
a point where a user would see something:

- CLI -> type a representative command, check the exit code and output.
- Server -> use an available permitted HTTP client for the changed route; inspect status, headers and body.
- TUI -> when tmux is available, send navigation keys and capture the pane result; otherwise report the unavailable interaction check.
- GUI -> click the button, screenshot the window. **Look at the
  screenshot.** A blank frame is a failure to launch.

If the fallback pattern didn't work out of the box - you had to
install packages, set env vars, patch config, or write a driver -
record the verified setup and launch recipe in project documentation or an authorized local skill. If it just worked, don't.
