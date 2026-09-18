pub fn run_init() -> std::process::ExitCode {
    let cwd = match std::env::current_dir() {
        Ok(p) => p,
        Err(e) => {
            eprintln!("cannot determine working directory: {e}");
            return std::process::ExitCode::FAILURE;
        }
    };

    let agent_dir = cwd.join("agent");
    let _ = std::fs::create_dir_all(&agent_dir);

    let config = cwd.join("agent.toml");
    if !config.exists() {
        let toml = r#"# Agent project configuration
mode = "interactive"
[model]
provider = "mock"
model = "mock-1"

[permissions]
mode = "ask"

[limits]
max_tool_calls = 50
max_parallel_tools = 4

[mcp]
[servers]
"#;
        match std::fs::write(&config, toml) {
            Ok(_) => println!("✓ created agent.toml"),
            Err(e) => {
                eprintln!("✗ failed to write agent.toml: {e}");
                return std::process::ExitCode::FAILURE;
            }
        }
    } else {
        println!("! agent.toml already exists; skipping");
    }

    let skills = agent_dir.join("skills");
    let _ = std::fs::create_dir_all(&skills);
    let skill_md = skills.join("my-first-skill").join("SKILL.md");
    if !skill_md.exists() {
        let _ = std::fs::create_dir_all(skill_md.parent().unwrap());
        let _ = std::fs::write(
            &skill_md,
            "---\ntools: read_file\n---\n# My First Skill\nWrite your skill instructions here.\n",
        );
        println!("✓ created starter skill at {}", skill_md.display());
    } else {
        println!("! starter skill already exists; skipping");
    }

    println!("\nProject initialized in {}", cwd.display());
    println!("Run 'agent' to start the interactive coding agent.");
    std::process::ExitCode::SUCCESS
}
