Please analyze this codebase and create or update **{{.Config.Options.InitializeAs}}** in the repository root to help future agents work effectively in this repository.

First check if the directory is empty or contains only config files. If so, stop and say: "Directory appears empty or only contains config. Add source code first, then run this command to generate {{.Config.Options.InitializeAs}}."

# Process

1. Check directory contents and repository structure (`ls`, `glob`).
2. Check for existing rule files (`.cursor/rules/*.md`, `.cursorrules`, `.github/copilot-instructions.md`, `CLAUDE.md`, `AGENTS.md`) and read `{{.Config.Options.InitializeAs}}` if it already exists so you can improve it rather than discarding useful notes.
3. Identify build, lint, test, and run commands from config files, manifests, Makefiles, or CI workflows.
4. Read representative source files to understand the architecture, module boundaries, conventions, and non-obvious patterns.

# What to include

- **Commands**: Essential commands to build, lint, test (including how to run a single test or targeted subset), and run the project.
- **Architecture**: High-level code organization, entry points, and how major components interact across files/packages ("big picture" that requires reading multiple files to understand).
- **Conventions & Gotchas**: Non-obvious naming rules, patterns, implicit constraints, or surprising tool/flag behaviours discovered in the codebase.
- **Existing Rules**: Relevant project-specific guidance from existing rule files.

# What NOT to include

- Generic software engineering advice or obvious standard practices.
- Long file-by-file or symbol-by-symbol inventories that are immediately obvious from `ls` or reading a single file.
- Unverified commands, paths, or conventions — only document what you directly observed.

Format with clear, concise Markdown sections.
