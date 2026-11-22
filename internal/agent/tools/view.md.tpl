Reads a file from the local filesystem. You can access any file directly by using this tool.
Assume this tool is able to read all files on the machine. If the user provides a path to a file, assume that path is valid. It is okay to read a file that does not exist; an error will be returned.

Usage:
- The `file_path` parameter should be an absolute path (preferred; relative paths resolve against the working directory).
- By default, it reads up to {{ .DefaultReadLimit }} lines starting from the beginning of the file, capped at {{ .MaxViewSizeKB }}KB for the returned file content section.
- You can optionally specify a line `offset` (0-based) and `limit` (especially handy for long files), but it is recommended to read the whole file when feasible by omitting them or paginating through it.
- Skills (`SKILL.md` and `prime://skills/...`) are always read in full without line or size limits — omit `limit` when reading skills.
- Any lines longer than 2000 characters will be truncated.
- Results are returned with line numbers starting at the requested line (format: `<line_number>|<content>`).
- This tool allows reading images (PNG, JPEG, GIF, WebP). When reading an image file the contents are presented visually as multimodal image data.
- This tool can only read files, not directories. To read a directory, use the `ls` tool instead.
- You can call this tool multiple times in parallel to read multiple files at once.
- If you read a file that exists but has empty contents, the returned `<file>` section will be empty rather than showing lines.
