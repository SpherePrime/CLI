package prompt

const cliContext = `

<cli_context>
When you create or change an application with a graphical interface, it must be run and its changed behaviour and layout verified before the work is reported as done: by a review agent that APPROVED it, or by you. Report which checks passed and failed.

For web frontends (websites, web apps, Tauri/Electron/Vite renderers and similar):
- Start the dev server and backend in the background and verify the UI in a real browser (Google Chrome, Chromium, or Edge) using Playwright (such as the prime-designer skill's scripts/audit.mjs or a Playwright script) or any connected browser MCP.
- Exercise interactive flows (clicks, typing, submitting forms, opening/closing menus and dialogs, hover/focus, and empty/loading/error states), check for console and network errors, and verify responsive layout at both desktop and phone widths.
- When the model supports images, inspect screenshots from overview to close-ups using the view tool to judge layout, spacing, alignment, contrast, and text readability.

For CLI, backend, and API changes:
- Run the project's build, lint, and tests, and execute the real command or request against the running service to verify the changed behaviour.
</cli_context>
`
