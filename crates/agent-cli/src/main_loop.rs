use std::process::ExitCode;
use std::sync::Arc;

use agent_config::{AgentConfig, ConfigLoader, ModelConfig};
use agent_model::{
    build_from_model_config, ChatMessage, MessageContent, ModelProvider, ModelRequest, Role,
};
use agent_ui::{provider_name, run_config_wizard, run_model_wizard};
use crossterm::event::{self, KeyCode, KeyModifiers};
use crossterm::terminal::ClearType;

pub struct AgentInput {
    pub input: String,
}

impl AgentInput {
    pub fn new() -> Self {
        Self { input: String::new() }
    }

    pub fn push_char(&mut self, c: char) {
        self.input.push(c);
    }

    pub fn pop(&mut self) {
        self.input.pop();
    }

    pub fn clear(&mut self) {
        self.input.clear();
    }
}

pub fn run_tui_app(storage: Option<agent_storage::Storage>) -> ExitCode {
    let _ = crossterm::execute!(std::io::stdout(), crossterm::terminal::Clear(ClearType::All));

    let mut app = agent_ui::AgentApp::new("Agent", env!("CARGO_PKG_VERSION"));
    let runtime = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .expect("tokio runtime creation failed");

    let mut cfg = ConfigLoader::new().load().unwrap_or_default();
    let mut model = cfg.model.clone().unwrap_or_else(default_model);
    let cwd = std::env::current_dir().unwrap_or_default();
    let _ = std::fs::create_dir_all(cwd.join(".agent/session"));

    app.set_model_config(model.clone());

    loop {
        match event::read() {
            Ok(event::Event::Key(key)) => {
                if key.code == KeyCode::Char('c') && key.modifiers.contains(KeyModifiers::CONTROL) {
                    println!("\n\x1b[31m  ^C — press Ctrl+D to quit\x1b[0m");
                    continue;
                }

                if let Some(event) = app.handle_event(key.code) {
                    match event {
                        agent_ui::AppEvent::Shutdown => {
                            if let Some(s) = storage {
                                let _ = s.write_session(
                                    &agent_storage::SessionRecord::new(
                                        Some(cwd.clone()),
                                        Some(model.model.clone()),
                                    ),
                                    &[],
                                );
                            }
                            return ExitCode::SUCCESS;
                        }
                        agent_ui::AppEvent::SendMessage(msg) => {
                            app.set_spinner(true, "thinking");
                            
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
                                        content: MessageContent::Text(msg),
                                        tool_calls: None,
                                        tool_call_id: None,
                                    },
                                ],
                                tools: None,
                                temperature: model.temperature,
                                max_tokens: model.max_tokens,
                            };

                            let provider = match build_provider(&model) {
                                Ok(p) => p,
                                Err(e) => {
                                    app.set_spinner(false, "");
                                    app.append_status(&format!("model init error: {}", e));
                                    continue;
                                }
                            };

                            let response = runtime.block_on(provider.chat(&request));
                            app.set_spinner(false, "");

                            match response {
                                Ok(resp) => {
                                    app.append_assistant(&resp.content.as_text());
                                    app.append_status("done");
                                }
                                Err(e) => {
                                    app.append_status(&format!("model error: {}", e));
                                }
                            }

                            if let Some(new_model) = &cfg.model {
                                model = new_model.clone();
                            }
                        }
                        agent_ui::AppEvent::SlashCommand(cmd) => {
                            handle_slash_command(&cmd, &mut cfg, &mut model, &mut app, &cwd);
                        }
                        agent_ui::AppEvent::StateChanged(new_state) => {
                            app.set_state(new_state);
                        }
                        _ => {}
                    }
                }
            }
            _ => {}
        }
    }
}

fn build_provider(model: &ModelConfig) -> anyhow::Result<Arc<dyn ModelProvider>> {
    Ok(Arc::from(build_from_model_config(model)?))
}

fn handle_slash_command(
    cmd: &str,
    cfg: &mut AgentConfig,
    model: &mut ModelConfig,
    app: &mut agent_ui::AgentApp,
    cwd: &std::path::Path,
) {
    match cmd {
        "/exit" | "/quit" => {
            if let Some(storage) = agent_storage::Storage::global().ok() {
                let _ = storage.write_session(
                    &agent_storage::SessionRecord::new(
                        Some(cwd.to_path_buf()),
                        Some(model.model.clone()),
                    ),
                    &[],
                );
            }
            std::process::exit(0);
        }
        "/help" | "/?" => {
            app.set_state(agent_ui::AppState::Banner);
        }
        "/model" => {
            let new = run_model_wizard(Some(model));
            *model = new;
            cfg.model = Some(model.clone());
            if let Ok(p) = ConfigLoader::save_global(cfg) {
                app.append_status(&format!("model updated, saved to {}", p.display()));
            } else {
                app.append_status("model updated (in-memory only)");
            }
        }
        "/config" | "/set" => {
            let new_cfg = run_config_wizard(cfg);
            *cfg = new_cfg;
            *model = cfg.model.clone().unwrap_or_else(default_model);
            app.set_model_config(model.clone());
            let _ = crossterm::execute!(std::io::stdout(), crossterm::terminal::Clear(ClearType::All));
            app.append_status("config saved");
        }
        "/status" | "/session" => {
            if cmd == "/status" {
                app.append_status(&format!(
                    "model: {} ({})  ·  cwd: {}",
                    model.model,
                    provider_name(&model.provider),
                    cwd.display()
                ));
            } else {
                let session_id = uuid::Uuid::new_v4();
                app.append_status(&format!(
                    "session: {}  ·  model: {}  ·  provider: {}",
                    session_id,
                    model.model,
                    provider_name(&model.provider)
                ));
            }
        }
        "/tools" => {
            app.append_status("tools: read_file, write_file, edit_file, shell, ...");
        }
        "/clear" => {
            let _ = crossterm::execute!(std::io::stdout(), crossterm::terminal::Clear(ClearType::All));
        }
        "/compact" | "/undo" | "/retry" | "/diff" => {
            app.append_status(&format!("{}: not implemented yet", cmd));
        }
        "/mcp" => {
            app.append_status("use `agent mcp list|add|...` in shell");
        }
        "/skills" => {
            app.append_status("use `agent skill list|install|...` in shell");
        }
        "/plugins" => {
            app.append_status("use `agent plugin list|install|...` in shell");
        }
        _ => {}
    }
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

pub fn run_main_loop(storage: Option<agent_storage::Storage>) -> ExitCode {
    run_tui_app(storage)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_agent_input_new() {
        let input = AgentInput::new();
        assert!(input.input.is_empty());
    }

    #[test]
    fn test_agent_input_push() {
        let mut input = AgentInput::new();
        input.push_char('a');
        input.push_char('b');
        input.push_char('c');
        assert_eq!(input.input, "abc");
    }

    #[test]
    fn test_agent_input_pop() {
        let mut input = AgentInput::new();
        input.push_char('a');
        input.push_char('b');
        input.pop();
        assert_eq!(input.input, "a");
    }

    #[test]
    fn test_agent_input_clear() {
        let mut input = AgentInput::new();
        input.push_char('a');
        input.push_char('b');
        input.clear();
        assert!(input.input.is_empty());
    }
}