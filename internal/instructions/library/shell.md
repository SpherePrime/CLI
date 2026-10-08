# Running shell commands

Run commands in the actual shell and working directory.
Prime's bash tool uses a Bash-compatible mvdan/sh interpreter on every platform, including Windows. Use Bash syntax and forward-slash paths; do not assume Windows has every POSIX utility installed. Use dedicated tools for files. Quote paths and literal input safely, including spaces, dollars, backticks and substitutions. Verify directories and targets before mutations. Run dependencies sequentially with failure handling; batch independent calls where supported. Observe exit status and output; do not hide failures.

Detailed references: read_instruction with `source-bash-overview`, `source-bash-quote-file-paths`, `source-bash-maintain-cwd`, `source-parallel-tool-call-note-part-of-tool-usage-policy`.
