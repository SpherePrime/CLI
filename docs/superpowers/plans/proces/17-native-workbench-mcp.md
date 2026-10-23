# Native project MCP parity

Status: implementation written; combined host compilation and focused tests queued after SSH fixtures.

Scope: port the existing project-bound automation and Git/GitHub tool definitions and handlers into the native MCP endpoint using Rust services. Preserve original names, schemas, workspace binding, revision checks and Go chat permission policy. Team/job functionality is outside this task. Do not advertise tools without working native handlers.

Implemented: all eight original automation tools, git_restore_commit, git_revert_commit and github_failed_logs. GitHub publication tools github_create_pull and github_comment wrap the existing native createPull/comment service actions; these actions existed in the original UI service but had no original MCP registrations. Create pull explicitly describes its origin push. No external publication was performed during implementation.

Automation adapters retain strict original argument fields, UUID IDs, default enabled=true, trigger variants and limits, project-bound listing, partial update/pause merging, native persistence/run/cancel behavior and the existing openSession renderer event. Remote mutations retain the native automation service's explicit unsupported error. Git operations use the bound local project only; remote projects cannot execute against their local cache. Restore/revert retain the native revision checks and backup behavior. Native tools remain behind the existing Go MCP permission policy.

Validation: original registrations and service schemas compared; rustfmt and git diff --check passed. Three new unit tests cover trigger rejection, partial updates/project rule isolation and Git action mapping/argument validation. The existing schema test now checks unique registrations, forbidden workspace/root overrides and required expectedHead for mutations. The parent previously verified native Git mutation safety fixtures in the combined test run. New MCP adapter tests and combined compilation remain queued. Team/job MCP tools are outside this task.
