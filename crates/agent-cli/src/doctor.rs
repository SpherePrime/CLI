use agent_storage::Storage;

pub fn run_doctor(storage: &Storage) -> std::process::ExitCode {
    let mut hard_failures = 0;

    let home = dirs::home_dir();
    if home.is_none() {
        eprintln!("✗ cannot determine home directory");
        hard_failures += 1;
    } else {
        println!("✓ home directory: {}", home.unwrap().display());
    }

    let cfg_path = storage.root().join("config.toml");
    if cfg_path.exists() {
        println!("✓ global config exists: {}", cfg_path.display());
    } else {
        println!("! global config not found (run 'agent init' or 'agent config')");
    }

    let provider = std::env::var("AGENT_API_KEY").is_ok()
        || std::env::var("OPENAI_API_KEY").is_ok()
        || std::env::var("ANTHROPIC_API_KEY").is_ok();
    if provider {
        println!("✓ API key configured");
    } else {
        println!("! no API key in environment; agent will run with mock provider");
    }

    let mcp_servers = agent_config::ConfigLoader::new()
        .load()
        .map(|c| c.mcp.servers.len())
        .unwrap_or(0);
    if mcp_servers > 0 {
        println!("✓ {mcp_servers} MCP server(s) configured");
    } else {
        println!("! no MCP servers configured");
    }

    let plugins_dir = storage.root().join("plugins");
    if plugins_dir.exists() {
        let n = std::fs::read_dir(&plugins_dir).map(|d| d.count()).unwrap_or(0);
        println!("✓ plugins dir: {} ({n} plugins)", plugins_dir.display());
    } else {
        let _ = std::fs::create_dir_all(&plugins_dir);
        println!("✓ plugins dir created: {}", plugins_dir.display());
    }

    let skills_dir = storage.root().join("skills");
    let _ = std::fs::create_dir_all(&skills_dir);
    println!("✓ skills dir: {}", skills_dir.display());

    let audit_dir = storage.audit_dir();
    println!("✓ audit log: {}", audit_dir.display());

    println!("\ndiagnostic: {hard_failures} hard failure(s) detected");
    if hard_failures > 0 {
        std::process::ExitCode::FAILURE
    } else {
        std::process::ExitCode::SUCCESS
    }
}
