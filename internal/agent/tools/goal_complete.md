Mark the chat goal as achieved. Only available when the user set a goal for this chat.

- Call it only after every stage in todos is completed and a final acceptance pass of the whole product found nothing left to fix.
- The final acceptance pass means: build and run the real product (websites in the built-in browser, native apps with app_* tools), go through every user flow including empty, invalid and error states, run all tests and linters, and review the full diff.
- The product must be production-ready: no placeholders, stubs, TODOs or mock data; errors and edge cases handled; clean, consistent interface.
- The summary is shown to the user: say briefly what was delivered and how it was verified.
- Never call it to stop early or to hand a question to the user; use the question tool for that.
