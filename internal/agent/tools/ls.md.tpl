Lists files and directories at a given path as a tree.
- Skips hidden files and common system directories; respects `.gitignore`; returns up to {{ .MaxFiles }} files.
- Supports optional `path` (defaults to the current working directory), `depth` (maximum traversal depth), and `ignore` (list of glob patterns to exclude).
- Prefer `glob` to find files by name patterns and `grep` to search file contents.
