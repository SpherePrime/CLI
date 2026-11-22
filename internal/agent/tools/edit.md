Performs exact string replacements in files.

Usage:
- You must read the file with `view` before editing.
- When editing text from `view` tool output, ensure you preserve the exact indentation (tabs/spaces) as it appears AFTER the line number prefix. The line number prefix format is spaces followed by a line number and `|` (e.g., `     1|`). Everything after that is the actual file content to match. Never include any part of the line number prefix in `old_string` or `new_string`.
- ALWAYS prefer editing existing files in the codebase. NEVER write new files unless explicitly required.
- Only use emojis if the user explicitly requests it. Avoid adding emojis to files unless asked.
- Keep `old_string` minimal — usually 1-3 lines, only enough to be unique in the file. Including excess context wastes tokens and is error-prone.
- The edit will FAIL if `old_string` is not unique in the file. Either provide a larger string with more surrounding context to make it unique, or use `replace_all: true` to change every instance of `old_string`.
- Use `replace_all: true` for replacing and renaming strings across the file (for example, renaming a variable).
- If `old_string` differs only in leading/trailing indentation, `edit` falls back to whitespace-normalized matching and notes when that happened.
- For whole-function, method, or type replacements prefer `lsp_replace_symbol` (no whitespace matching needed). For semantic cross-file renames prefer `lsp_rename`.
- Omit `old_string` (empty) only when creating a new file, or omit `new_string` (empty) to delete `old_string`.
