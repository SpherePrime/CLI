A powerful search tool built on ripgrep (`rg`) with native regex fallback.

Usage:
- ALWAYS use `grep` for search tasks. NEVER invoke `grep` or `rg` as a `bash` command. The `grep` tool is optimized for file searches and respects `.gitignore`.
- Supports full regex syntax (e.g., `log.*Error`, `function\s+\w+`). Set `literal_text: true` to treat the pattern as literal text instead of regex.
- Filter files with the `include` parameter (e.g., `*.js`, `*.{ts,tsx}`).
- Returns matching file paths and matching lines sorted by file modification time (max {{ .MaxResults }} results; respects `.gitignore`).
- Use `glob` when you need to find files by name patterns rather than file contents.
- When you are doing an open-ended search that may require multiple rounds of globbing and grepping, use the `agent` tool instead.
