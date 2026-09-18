use agent_config::McpServerConfig;

use crate::cli::{load_config, save_config, McpAction};

pub fn run(action: &McpAction) -> std::process::ExitCode {
    match action {
        McpAction::List => run_list(),
        McpAction::Add {
            name,
            command,
            args,
        } => run_add(name, command, args),
        McpAction::Remove { name } => run_remove(name),
        McpAction::Enable { name } => run_set_enabled(name, true),
        McpAction::Disable { name } => run_set_enabled(name, false),
        McpAction::Inspect { name } => run_inspect(name),
    }
}

fn run_list() -> std::process::ExitCode {
    let cfg = load_config();
    for (name, server) in &cfg.mcp.servers {
        println!(
            "{name} {}",
            if server.enabled {
                "[enabled]"
            } else {
                "[disabled]"
            }
        );
    }
    std::process::ExitCode::SUCCESS
}

fn run_add(name: &str, command: &str, args: &[String]) -> std::process::ExitCode {
    let mut cfg = load_config();
    cfg.mcp.servers.insert(
        name.to_string(),
        McpServerConfig {
            name: name.to_string(),
            command: Some(command.to_string()),
            args: args.to_vec(),
            url: None,
            env: Default::default(),
            enabled: true,
        },
    );
    match save_config(&cfg) {
        Ok(p) => {
            println!("added MCP server '{name}' to {}", p.display());
            std::process::ExitCode::SUCCESS
        }
        Err(e) => {
            eprintln!("failed to save config: {e}");
            std::process::ExitCode::FAILURE
        }
    }
}

fn run_remove(name: &str) -> std::process::ExitCode {
    let mut cfg = load_config();
    cfg.mcp.servers.remove(name);
    match save_config(&cfg) {
        Ok(p) => {
            println!("removed MCP server '{name}' from {}", p.display());
            std::process::ExitCode::SUCCESS
        }
        Err(e) => {
            eprintln!("failed to save config: {e}");
            std::process::ExitCode::FAILURE
        }
    }
}

fn run_set_enabled(name: &str, enabled: bool) -> std::process::ExitCode {
    let mut cfg = load_config();
    if let Some(s) = cfg.mcp.servers.get_mut(name) {
        s.enabled = enabled;
        println!(
            "{} '{}'",
            if enabled { "enabled" } else { "disabled" },
            name
        );
    } else {
        eprintln!("MCP server '{name}' not found");
        return std::process::ExitCode::FAILURE;
    }
    save_config(&cfg)
        .map(|_| std::process::ExitCode::SUCCESS)
        .unwrap_or_else(|_| std::process::ExitCode::FAILURE)
}

fn run_inspect(name: &str) -> std::process::ExitCode {
    let cfg = load_config();
    match cfg.mcp.servers.get(name) {
        Some(s) => {
            println!("{}", serde_json::to_string_pretty(s).unwrap_or_default());
            std::process::ExitCode::SUCCESS
        }
        None => {
            eprintln!("MCP server '{name}' not found");
            std::process::ExitCode::FAILURE
        }
    }
}
