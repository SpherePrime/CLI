# Tool Description Edit

Apply this reference only to its relevant task and within current user, project and mode instructions. Tool schemas determine supported parameters.

Performs exact string replacements in files.

Usage:
- Read the file with view before editing.
- When editing text from view tool output, ensure you preserve the exact indentation (tabs/spaces) as it appears AFTER the line number prefix. The line number prefix format is: the line-number decoration returned by view. Everything after that is the actual file content to match. Never include any part of the line number prefix in the old_string or new_string.
- ALWAYS prefer editing existing files in the codebase. NEVER write new files unless explicitly required.
- Only use emojis if the user explicitly requests it. Avoid adding emojis to files unless asked.
- Use the replacement options supported by edit or multiedit for replacing and renaming strings across the file. This parameter is useful if you want to rename a variable for instance.
