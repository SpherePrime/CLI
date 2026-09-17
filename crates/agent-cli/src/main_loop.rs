use std::process::ExitCode;
use std::sync::Arc;

use agent_config::{AgentConfig, ConfigLoader, ModelConfig};
use agent_model::{
    build_from_model_config, ChatMessage, MessageContent, ModelProvider, ModelRequest, Role,
};
use agent_ui::{provider_name, run_config_wizard, run_model_wizard, InputScreen};
use crossterm::event::{self, KeyCode, KeyModifiers};
use crossterm::terminal::ClearType;

pub struct AgentInput {
    pub input_screen: InputScreen,
}

impl AgentInput {
    pub fn new() -> Self {
        Self {
            input_screen: InputScreen::new(),
        }
    }

    pub fn try_complete(&mut self) {
        let prefix = self.input_screen.input.clone();
        let _ = self.input_screen.complete(&prefix);
    }
}

pub fn run_main_loop(storage: Option<agent_storage::Storage>) -> ExitCode {
    let runtime = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .expect("tokio runtime creation failed");

    let mut cfg = ConfigLoader::new().load().unwrap_or_default();
    let mut model = cfg.model.clone().unwrap_or_else(default_model);
    let _provider = build_provider(&model).unwrap_or_else(|_| {
        build_provider(&default_model()).expect("mock provider should always build")
    });

    let cwd = std::env::current_dir().unwrap_or_default();
    let _ = std::fs::create_dir_all(cwd.join(".agent/session"));

    let mut input = AgentInput::new();

    let _ = crossterm::execute!(
        std::io::stdout(),
        crossterm::terminal::Clear(ClearType::All)
    );
    draw_banner(&model, &cwd, env!("CARGO_PKG_VERSION"));

    loop {
        print!("{} ", colored_prompt());

        let mut line = String::new();
        loop {
            match event::read() {
                Ok(event::Event::Key(key)) => {
                    if key.code == KeyCode::Char('c') && key.modifiers.contains(KeyModifiers::CONTROL) {
                        println!("\n\x1b[31m  ^C — press /exit to quit\x1b[0m");
                        continue;
                    }

                    if let KeyCode::Char('_') = key.code {
                        if key.modifiers.contains(KeyModifiers::CONTROL) {
                            input.try_complete();
                            continue;
                        }
                    }

                    if let Some(result) = input.input_screen.handle_input(key.code, key.modifiers) {
                        line = result.to_string();
                        break;
                    }
                }
                Ok(event::Event::Key(key)) if key.code == KeyCode::Char('c') && key.modifiers.contains(KeyModifiers::CONTROL) => {
                    println!("\n\x1b[31m  ^C — press /exit to quit\x1b[0m");
                    continue;
                }
                _ => {}
            }
        }

        let trimmed = line.trim().to_string();
        if trimmed.is_empty() {
            continue;
        }

        print_user(&trimmed);
        match handle_command(&mut cfg, &mut model, &runtime, &trimmed, &cwd) {
            CmdOutcome::Continue => {}
            CmdOutcome::Quit => { break; }
        }
    }

    if let Some(storage) = &storage {
        let _ = storage.write_session(
            &agent_storage::SessionRecord::new(
                Some(cwd.clone()),
                Some(model.model.clone()),
            ),
            &[],
        );
    }

    print_goodbye();
    ExitCode::SUCCESS
}

fn handle_command(
    cfg: &mut AgentConfig,
    model: &mut ModelConfig,
    runtime: &tokio::runtime::Runtime,
    input: &str,
    cwd: &std::path::Path,
) -> CmdOutcome {
    match input {
        "/exit" | "/quit" => {
            return CmdOutcome::Quit;
        }
        "/help" | "/?" => {
            print_help();
            return CmdOutcome::Continue;
        }
        "/config" | "/set" => {
            let new_cfg = run_config_wizard(cfg);
            *cfg = new_cfg;
            *model = cfg.model.clone().unwrap_or_else(default_model);
            let _ = crossterm::execute!(
                std::io::stdout(),
                crossterm::terminal::Clear(ClearType::All)
            );
            draw_banner(model, cwd, env!("CARGO_PKG_VERSION"));
            print_status_line("config saved", "green");
            return CmdOutcome::Continue;
        }
        "/model" => {
            let new = run_model_wizard(Some(model));
            *model = new;
            cfg.model = Some(model.clone());
            if let Ok(p) = ConfigLoader::save_global(cfg) {
                print_status_line(&format!("✓ model updated, saved to {}", p.display()), "green");
            } else {
                print_status_line("✓ model updated (in-memory only)", "green");
            }
            let _ = crossterm::execute!(
                std::io::stdout(),
                crossterm::terminal::Clear(ClearType::All)
            );
            draw_banner(model, cwd, env!("CARGO_PKG_VERSION"));
            return CmdOutcome::Continue;
        }
        "/status" => {
            print_status_line(
                &format!(
                    "model: {} ({})  ·  cwd: {}",
                    model.model,
                    provider_name(&model.provider),
                    cwd.display()
                ),
                "dim",
            );
            return CmdOutcome::Continue;
        }
        "/session" => {
            let session_id = uuid::Uuid::new_v4();
            print_status_line(
                &format!(
                    "session: {}  ·  model: {}  ·  provider: {}",
                    session_id,
                    model.model,
                    provider_name(&model.provider)
                ),
                "dim",
            );
            return CmdOutcome::Continue;
        }
        "/tools" => {
            print_tools();
            return CmdOutcome::Continue;
        }
        "/clear" => {
            let _ = crossterm::execute!(
                std::io::stdout(),
                crossterm::terminal::Clear(ClearType::All)
            );
            draw_banner(model, cwd, env!("CARGO_PKG_VERSION"));
            return CmdOutcome::Continue;
        }
        "/compact" | "/undo" | "/retry" | "/diff" => {
            print_status_line(&format!("{input}: not implemented yet"), "yellow");
            return CmdOutcome::Continue;
        }
        "/mcp" => {
            print_status_line(
                "use `agent mcp list|add|enable|disable|inspect <name>` in a shell",
                "dim",
            );
            return CmdOutcome::Continue;
        }
        "/skills" => {
            print_status_line(
                "use `agent skill list|install|enable|disable <name>` in a shell",
                "dim",
            );
            return CmdOutcome::Continue;
        }
        "/plugins" => {
            print_status_line(
                "use `agent plugin list|install|enable|disable <name>` in a shell",
                "dim",
            );
            return CmdOutcome::Continue;
        }
        _ => {
            let request = ModelRequest {
                model: model.model.clone(),
                messages: vec![
                    ChatMessage {
                        role: Role::System,
                        content: MessageContent::Text(
                            "You are a production-grade AI coding agent.".into(),
                        ),
                        tool_calls: None,
                        tool_call_id: None,
                    },
                    ChatMessage {
                        role: Role::User,
                        content: MessageContent::Text(input.to_string()),
                        tool_calls: None,
                        tool_call_id: None,
                    },
                ],
                tools: None,
                temperature: model.temperature,
                max_tokens: model.max_tokens,
            };

            let provider = match build_provider(model) {
                Ok(p) => p,
                Err(e) => {
                    print_status_line(&format!("model init: {e}"), "red");
                    return CmdOutcome::Continue;
                }
            };

            print_status_line("⠋ thinking…", "dim");
            let response = runtime.block_on(provider.chat(&request));

            match response {
                Ok(resp) => {
                    print_assistant(&resp.content.as_text());
                }
                Err(e) => {
                    print_status_line(&format!("model error: {e}"), "red");
                }
            }
            return CmdOutcome::Continue;
        }
    }
}

fn build_provider(model: &ModelConfig) -> anyhow::Result<Arc<dyn ModelProvider>> {
    Ok(Arc::from(build_from_model_config(model)?))
}

fn storage_root() -> std::path::PathBuf {
    match agent_storage::Storage::global() {
        Ok(s) => s.root().clone(),
        Err(_) => std::path::PathBuf::from(".agent"),
    }
}

enum CmdOutcome {
    Continue,
    Quit,
}

fn colored_prompt() -> String {
    format!("\x1b[1;38;2;79;193;255m❯ \x1b[0m")
}

fn draw_banner(model: &ModelConfig, cwd: &std::path::Path, version: &str) {
    let line = "══════════════════════════════════════════════";
    println!("\x1b[1;38;2;79;193;255m{line}\x1b[0m");
    println!(
        "\x1b[1;38;2;79;193;255m  AGENT  —  AI Coding Agent  v{version}\x1b[0m"
    );
    println!(
        "\x1b[38;2;107;114;128m  model: {}  ·  provider: {}  ·  cwd: {}\x1b[0m",
        model.model,
        provider_name(&model.provider),
        cwd.display()
    );
    println!(
        "\x1b[38;2;107;114;128m  /help · /model · /config · /status · /tools · /exit\x1b[0m"
    );
    println!("\x1b[1;38;2;79;193;255m{line}\x1b[0m");
}

fn print_user(text: &str) {
    println!("\n\x1b[1;34m  👤 you\x1b[0m  \x1b[34m{text}\x1b[0m");
}

fn print_assistant(text: &str) {
    println!("\n\x1b[1;36m  🤖 agent\x1b[0m");
    let lines: Vec<&str> = text.lines().collect();
    if lines.is_empty() {
        println!("    {text}");
    } else {
        for l in &lines {
            println!("    {l}");
        }
    }
    println!();
}

fn print_status_line(text: &str, color: &str) {
    let code = match color {
        "red" => 31,
        "green" => 32,
        "yellow" => 33,
        "blue" => 34,
        "cyan" => 36,
        _ => 90,
    };
    println!("\x1b[{code}m{text}\x1b[0m");
}

fn print_help() {
    print_status_line("─── commands ────", "cyan");
    for (cmd, desc) in [
        ("/help", "show this help"),
        (
            "/model",
            "wizard: change model, provider, base_url, api_key, temperature",
        ),
        ("/config", "config menu: model, permissions, limits, save"),
        ("/status", "show current model, provider, working dir"),
        ("/session", "show session metadata"),
        ("/tools", "list available tools"),
        ("/clear", "clear screen"),
        ("/compact", "compact context (stub)"),
        ("/undo", "undo last change (stub)"),
        ("/exit", "quit and save session"),
    ] {
        print_status_line(
            &format!(
                "  \x1b[33m{cmd:<10}\x1b[0m \x1b[90m{desc}\x1b[0m"
            ),
            "dim",
        );
    }
    print_status_line("anything else is sent to the model", "dim");
}

fn print_tools() {
    print_status_line("─── tools ────", "cyan");
    for tool in [
        ("read_file", "read a file"),
        ("write_file", "write a file"),
        ("edit_file", "search/replace, line-range, patch"),
        ("delete_file", "delete a file"),
        ("list_directory", "list a directory"),
        ("search_files", "glob search"),
        ("find_files", "find files"),
        ("shell", "run shell command"),
        ("run_command", "run command"),
    ] {
        print_status_line(
            &format!(
                "  \x1b[32m{:<16}\x1b[0m \x1b[90m{}\x1b[0m",
                tool.0, tool.1
            ),
            "dim",
        );
    }
}

fn print_goodbye() {
    print_status_line("┌────────────────────────────────────┐", "cyan");
    print_status_line("│         see you next time          │", "dim");
    print_status_line("└────────────────────────────────────┘", "cyan");
    print_status_line("session saved.", "dim");
}

fn default_model() -> ModelConfig {
    ModelConfig {
        provider: agent_config::ProviderKind::Mock,
        model: "mock-1".into(),
        base_url: None,
        api_key_env: None,
        temperature: None,
        max_tokens: None,
    }
}



#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_agent_input_new() {
        let input = AgentInput::new();
        assert!(input.input_screen.input.is_empty());
    }

    #[test]
    fn test_agent_input_try_complete() {
        let mut input = AgentInput::new();
        input.try_complete();
    }
}
