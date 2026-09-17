use std::io::{self, BufRead, Write};
use std::process::ExitCode;
use std::sync::Arc;

use agent_config::{AgentConfig, ConfigLoader, ProviderKind};
use agent_model::{build_from_model_config, ChatMessage, MessageContent, ModelProvider, ModelRequest, Role};
use agent_ui::spinner::Spinner;
use agent_ui::status_bar::StatusBar;

pub fn run_main_loop(storage: Option<agent_storage::Storage>) -> ExitCode {
    let runtime = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .expect("tokio runtime creation failed");

    let mut cfg = ConfigLoader::new().load().unwrap_or_default();
    let model = cfg.model.clone().unwrap_or_else(default_model);
    let provider: Arc<dyn ModelProvider> = match build_from_model_config(&model) {
        Ok(p) => Arc::from(p),
        Err(e) => {
            eprintln!("[model] {e}");
            return ExitCode::FAILURE;
        }
    };

    let cwd = std::env::current_dir().unwrap_or_default();
    let status_bar = StatusBar::new()
        .with_model(&model.model)
        .with_project(&cwd)
        .with_mode("interactive");

    print_banner(&model);

    let mut input = String::new();
    let mut spinner = Spinner::new();
    let mut active_input = String::new();

    loop {
        print_prompt();
        io::stdout().flush().ok();
        input.clear();
        match io::stdin().lock().read_line(&mut input) {
            Ok(0) => break,
            Err(e) => {
                eprintln!("[read] {e}");
                break;
            }
            Ok(_) => {}
        }
        let user_input = input.trim().to_string();
        if user_input.is_empty() {
            continue;
        }

        print_user(&user_input);
        match user_input.as_str() {
            "/exit" | "/quit" => {
                print_goodbye();
                break;
            }
            "/clear" => {
                print_status_line("screen cleared", "dim");
                continue;
            }
            "/help" => {
                print_help();
                continue;
            }
            "/tools" => {
                print_tools();
                continue;
            }
            "/status" => {
                print_status_line(&status_bar.to_string(), "dim");
                continue;
            }
            "/config" | "/set" => {
                cfg = run_config_menu(cfg, &runtime, &provider);
                let model = cfg.model.clone().unwrap_or_else(default_model);
                let _ = &model;
                continue;
            }
            "/model" => {
                print_status_line("model: ", "dim");
                if let Some(mc) = &cfg.model {
                    print_status_line(&format!("{} ({})", mc.model, provider_name(&mc)), "cyan");
                } else {
                    print_status_line("(none configured)", "yellow");
                }
                continue;
            }
            "/session" => {
                print_status_line(
                    &format!(
                        "session: {} | model: {} | provider: {}",
                        session_id_label(),
                        model.model,
                        provider_name(&model)
                    ),
                    "dim",
                );
                continue;
            }
            "/compact" => {
                print_status_line("context compacted (stub)", "dim");
                continue;
            }
            "/undo" => {
                print_status_line("undo: not yet implemented", "yellow");
                continue;
            }
            "/retry" => {
                print_status_line("retry: not yet implemented", "yellow");
                continue;
            }
            _ => {}
        }

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
                    content: MessageContent::Text(user_input.clone()),
                    tool_calls: None,
                    tool_call_id: None,
                },
            ],
            tools: None,
            temperature: model.temperature,
            max_tokens: model.max_tokens,
        };

        spinner.start();
        let response = runtime.block_on(provider.chat(&request));
        spinner.stop();

        match response {
            Ok(resp) => {
                print_assistant(&resp.content.as_text());
            }
            Err(e) => {
                print_status_line(&format!("model error: {e}"), "red");
            }
        }
        active_input = user_input;
        let _ = active_input;
    }

    if let Some(storage) = &storage {
        let _ = storage.write_session(
            &agent_storage::SessionRecord::new(Some(cwd), Some(model.model)),
            &[],
        );
    }
    print_status_line("session saved. goodbye.", "dim");
    ExitCode::SUCCESS
}

fn session_id_label() -> String {
    uuid::Uuid::new_v4().to_string()
}

fn run_config_menu(
    mut cfg: AgentConfig,
    _runtime: &tokio::runtime::Runtime,
    _provider: &Arc<dyn ModelProvider>,
) -> AgentConfig {
    print_status_line("configuration (type /exit in value fields to cancel editing)", "dim");
    let mut model = cfg.model.clone().unwrap_or_else(default_model);

    loop {
        print_config_options(&model);
        let mut line = String::new();
        io::stdout().flush().ok();
        if io::stdin().read_line(&mut line).is_err() {
            break;
        }
        let choice = line.trim();
        match choice {
            "1" => {
                print_status_line("set provider: openai, anthropic, google, mock", "dim");
                if let Ok(p) = read_line() {
                    model.provider = match p.trim().to_lowercase().as_str() {
                        "anthropic" => ProviderKind::Anthropic,
                        "google" => ProviderKind::Google,
                        "mock" => ProviderKind::Mock,
                        "custom" => ProviderKind::Custom,
                        "openai" | _ => ProviderKind::OpenAi,
                    };
                    print_status_line("provider updated", "green");
                }
            }
            "2" => {
                print_status_line("set model name", "dim");
                if let Ok(m) = read_line() {
                    model.model = m.trim().to_string();
                    print_status_line("model updated", "green");
                }
            }
            "3" => {
                print_status_line("set temperature (0.0-2.0, empty to clear)", "dim");
                if let Ok(t) = read_line() {
                    model.temperature = if t.trim().is_empty() {
                        None
                    } else {
                        t.trim().parse().ok()
                    };
                    print_status_line("temperature updated", "green");
                }
            }
            "4" => {
                print_status_line("set max_tokens (empty to clear)", "dim");
                if let Ok(m) = read_line() {
                    model.max_tokens = if m.trim().is_empty() {
                        None
                    } else {
                        m.trim().parse().ok()
                    };
                    print_status_line("max_tokens updated", "green");
                }
            }
            "5" => {
                print_status_line("set base_url (empty to clear)", "dim");
                if let Ok(u) = read_line() {
                    model.base_url = if u.trim().is_empty() {
                        None
                    } else {
                        Some(u.trim().to_string())
                    };
                    print_status_line("base_url updated", "green");
                }
            }
            "6" => {
                let path = match agent_config::ConfigLoader::save_global(&{
                    let mut c = cfg.clone();
                    c.model = Some(model.clone());
                    c
                }) {
                    Ok(p) => p,
                    Err(e) => {
                        print_status_line(&format!("save failed: {e}"), "red");
                        continue;
                    }
                };
                print_status_line(&format!("saved to {}", path.display()), "green");
            }
            "7" | "q" | "exit" | "/exit" => {
                break;
            }
            _ => {
                print_status_line("choose 1-7 or q to quit config", "yellow");
            }
        }
    }
    cfg.model = Some(model);
    cfg
}

fn print_config_options(model: &agent_config::ModelConfig) {
    print_status_line("─ config ─────────────────────────────", "cyan");
    print_status_line(
        &format!(
            "  1. provider: {:?}  2. model: {}  3. temp: {:?}  4. max_tokens: {:?}  5. base_url: {:?}  6. save  7. quit",
            provider_name(model), model.model, model.temperature, model.max_tokens, model.base_url.as_deref().unwrap_or("(none)")
        ),
        "dim",
    );
}

fn read_line() -> Result<String, io::Error> {
    let mut buf = String::new();
    io::stdin().read_line(&mut buf).map(|_| buf.trim().to_string())
}

fn print_banner(model: &agent_config::ModelConfig) {
    let width = 60;
    let line = "═".repeat(width);
    let pad = |s: &str| {
        let l = s.len();
        if l >= width { s.to_string() } else {
            let total = width - l;
            let left = total / 2;
            " ".repeat(left) + s + &" ".repeat(total - left)
        }
    };

    print!("{}", colored(&line, "cyan"));
    println!();
    println!("{}", colored(&pad("  AGENT"), "bold"));
    println!(
        "{}",
        colored(&pad(&format!("  v{} · model: {} ({})", env!("CARGO_PKG_VERSION"), model.model, provider_name(model))), "dim")
    );
    println!(
        "{}",
        colored(&pad("  type /help for commands, /config to tune settings"), "dim")
    );
    print!("{}", colored(&line, "cyan"));
    println!();
}

fn print_prompt() {
    print!("{}", colored("❯ ", "cyan"));
}

fn print_user(text: &str) {
    println!("\n{} {}", colored("👤 you", "bold"), colored(text, "blue"));
}

fn print_assistant(text: &str) {
    println!("\n{} {}", colored("🤖 agent", "bold"), colored(text, "white"));
}

fn print_status_line(text: &str, color: &str) {
    println!("{}", colored(text, color));
}

fn print_help() {
    print_status_line("─── commands ────", "cyan");
    for (cmd, desc) in [
        ("/help", "show this help"),
        ("/config", "open configuration menu"),
        ("/model", "show current model"),
        ("/status", "show session status"),
        ("/session", "show session info"),
        ("/tools", "list available tools"),
        ("/clear", "clear screen"),
        ("/compact", "compact context (stub)"),
        ("/undo", "undo last change (stub)"),
        ("/exit", "quit"),
    ] {
        print_status_line(&format!("  {} — {}", colored(cmd, "yellow"), desc), "dim");
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
        print_status_line(&format!("  {} — {}", colored(tool.0, "green"), tool.1), "dim");
    }
}

fn print_goodbye() {
    print_status_line("┌────────────────────────────────────┐", "cyan");
    print_status_line("│        see you next time          │", "dim");
    print_status_line("└────────────────────────────────────┘", "cyan");
}

fn colored(s: &str, color: &str) -> String {
    let code = match color {
        "red" => "31",
        "green" => "32",
        "yellow" => "33",
        "blue" => "34",
        "magenta" => "35",
        "cyan" => "36",
        "white" => "37",
        "dim" => "90",
        _ => "0",
    };
    let style = if color == "bold" { "1;36" } else { code };
    format!("\x1b[{style}m{}\x1b[0m", s)
}

fn provider_name(mc: &agent_config::ModelConfig) -> &str {
    match mc.provider {
        ProviderKind::OpenAi => "openai",
        ProviderKind::Anthropic => "anthropic",
        ProviderKind::Google => "google",
        ProviderKind::OpenAiCompatible => "openai-compatible",
        ProviderKind::Custom => "custom",
        ProviderKind::Mock => "mock",
    }
}

fn default_model() -> agent_config::ModelConfig {
    agent_config::ModelConfig {
        provider: ProviderKind::Mock,
        model: "mock-1".into(),
        base_url: None,
        api_key_env: None,
        temperature: None,
        max_tokens: None,
    }
}
