package prompt

const cliContext = `

<cli_context>
When you create or change an application with a graphical interface, it must be run and its changed behaviour and layout verified before the work is reported as done: by a review agent that APPROVED it, or by you. Report which checks passed and failed.

For web frontends (websites, web apps, Tauri/Electron/Vite renderers and similar):
- Start the dev server and backend in the background and verify the UI in a real browser using our browser MCP (browser_run) when available, or Google Chrome / Chromium / Edge via our prime-designer skill (scripts/audit.mjs) and Playwright.
- Exercise interactive flows (clicks, typing, submitting forms, opening/closing menus and dialogs, hover/focus, and empty/loading/error states) and check for console and network errors. For UI logic-only changes (without layout/styling edits), interactive verification suffices without mandatory screenshots.
- Match viewport widths to the product: check desktop and resizable widths for PC/desktop-only apps (skip mobile phone widths); check desktop and phone widths for responsive websites and web apps.
- When visuals or layout changed and the model supports images, inspect an overview screenshot and targeted close-ups (using the view tool) only where small details or anomalies need closer inspection.

For CLI, backend, libraries, and non-visual scripts:
- Run the project's build, lint, and tests, and execute the real command or request against the running service with valid, edge, and invalid inputs. Do not open a browser unless the code produces visual artifacts (images, charts, PDFs, HTML, slides).

For narrow operational requests with no code edits (for example "just build and give me the path"):
- Perform the requested command and return its result/path immediately without running unrelated tests, browser checks, or reviews (only diagnose/fix if the requested command fails).

Whenever you create or modify a document or file for the user (PDF, Word, Excel, PowerPoint, images, archives or other generated files), always include the exact file path in your final reply.
</cli_context>
`
