# Tools

Tools are registered with the `ToolRegistry` and callable by name.

## Built-in

- `read_file` — read a text file
- `write_file` — overwrite a file
- `edit_file` — search/replace, line-range, or patch edits with snapshot
- `delete_file` — remove a file (permission-gated)
- `list_directory` — list entries
- `search_files` / `find_files` — glob-based search
- `shell` / `run_command` — execute commands in the working dir
- `git_*` — status, diff, log, branch, checkout, add, commit

## Parallel execution

Independent tool calls can be dispatched in parallel (`call_parallel`), with a configurable max concurrency.

## Custom tools

Implement `ToolExecutor` in a plugin and register it with the `ToolRegistry`.
