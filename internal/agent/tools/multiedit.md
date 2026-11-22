Performs multiple sequential exact string replacements on a single file in one operation. Each edit operates on the result of the previous edit.

Usage:
- Prefer `multiedit` over multiple `edit` calls when making several changes to the same file.
- You must read the file with `view` before editing.
- When editing text from `view` tool output, ensure you preserve the exact indentation (tabs/spaces) as it appears AFTER the line number prefix (`<spaces><number>|`). Never include any part of the line number prefix in `old_string` or `new_string`.
- ALWAYS prefer editing existing files in the codebase. NEVER write new files unless explicitly required.
- Only use emojis if the user explicitly requests it. Avoid adding emojis to files unless asked.
- Keep each `old_string` minimal (usually 1-3 lines) and unique at that step in the sequence, or set `replace_all: true` to replace every occurrence in that step.
- Plan edits carefully so earlier replacements do not break later `old_string` matches.
- For whole-function, method, or type replacements prefer `lsp_replace_symbol`. For semantic cross-file renames prefer `lsp_rename`.
