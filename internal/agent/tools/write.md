Writes a file to the local filesystem, creating a new file or overwriting an existing one (auto-creates parent directories).

Usage:
- If this is an existing file, you MUST read it with the `view` tool first — overwriting an existing file you have not read in this session (or that changed since it was last read) will fail.
- ALWAYS prefer editing existing files in the codebase with `edit` or `multiedit` over overwriting them with `write`. Only use `write` when creating a new file or replacing an entire file you have already read.
- NEVER proactively create documentation files (`*.md`) or README files. Only create documentation files if explicitly requested by the user.
- Only use emojis if the user explicitly requests it. Avoid writing emojis to files unless asked.
