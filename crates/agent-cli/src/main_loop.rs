use std::process::ExitCode;
use std::sync::Arc;

use agent_config::{AgentConfig, ConfigLoader, ModelConfig};
use agent_model::{
    build_from_model_config, ChatMessage, MessageContent, ModelProvider, ModelRequest, Role,
};
use agent_ui::{provider_name, run_config_wizard, run_model_wizard, AppEvent, AppState, AgentApp, StatusType};
use crossterm::event::{self, KeyCode, KeyModifiers};
use crossterm::terminal::ClearType;
use ratatui::layout::Layout;

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
    use ratatui::Terminal;
    use ratatui::backend::CrosstermBackend;
    
    let mut terminal = Terminal::new(CrosstermBackend::new(std::io::stdout()))
        .expect("failed to create terminal");
    
    let mut app = agent_ui::AgentApp::new("AI Coding Agent", env!("CARGO_PKG_VERSION"));
    let runtime = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .expect("tokio runtime creation failed");

    let mut cfg = ConfigLoader::new().load().unwrap_or_default();
    let mut model = cfg.model.clone().unwrap_or_else(default_model);

    app.set_model_config(model.clone());

    let _ = crossterm::execute!(std::io::stdout(), crossterm::terminal::Clear(ClearType::All));

    loop {
        // Render the UI
        let _ = terminal.draw(|f| {
            app.render(f, Layout::default());
        });

        match event::read() {
            Ok(event::Event::Key(key)) => {
                if key.code == KeyCode::Char('c') && key.modifiers.contains(KeyModifiers::CONTROL) {
                    println!("\n\x1b[31m  ^C — press Ctrl+D to quit\x1b[0m");
                    continue;
                }

                if let Some(ev) = app.handle_event(key.code) {
                    match ev {
                        agent_ui::AppEvent::Shutdown => {
                            if let Some(s) = storage {
                                let _ = s.write_session(
                                    &agent_storage::SessionRecord::new(
                                        Some(std::env::current_dir().unwrap_or_default()),
                                        Some(model.model.clone()),
                                    ),
                                    &[],
                                );
                            }
                            return ExitCode::SUCCESS;
                        }
                        agent_ui::AppEvent::SendMessage(msg) => {
                            app.status = agent_ui::StatusType::Thinking("thinking".to_string());

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
                                    app.status = agent_ui::StatusType::Error(format!("model init error: {}", e));
                                    std::thread::sleep(std::time::Duration::from_millis(100));
                                    continue;
                                }
                            };

                            let response = runtime.block_on(provider.chat(&request));
                            app.status = agent_ui::StatusType::Success;

                            match response {
                                Ok(resp) => {
                                    app.push_assistant(&resp.content.as_text());
                                }
                                Err(e) => {
                                    app.status = agent_ui::StatusType::Error(format!("model error: {}", e));
                                }
                            }

                            if let Some(new_model) = &cfg.model {
                                model = new_model.clone();
                            }
                        }
                        agent_ui::AppEvent::SlashCommand(cmd) => {
                            handle_slash_command(&cmd, &mut cfg, &mut model, &mut app);
                        }
                        agent_ui::AppEvent::StateChanged(new_state) => {
                            app.current_state = new_state;
                        }
                        agent_ui::AppEvent::Undo => {
                            app.undo_last();
                        }
                        agent_ui::AppEvent::Clear => {
                            app.clear_messages();
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
) {
    match cmd {
        "/exit" | "/quit" => {
            std::process::exit(0);
        }
        "/help" | "/?" => {
            app.current_state = agent_ui::AppState::Banner;
        }
        "/model" => {
            let new = run_model_wizard(Some(model));
            *model = new;
            cfg.model = Some(model.clone());
            if let Ok(_p) = ConfigLoader::save_global(cfg) {
                app.status = agent_ui::StatusType::Success;
            } else {
                app.status = agent_ui::StatusType::Error("failed to save config".to_string());
            }
        }
        "/config" | "/set" => {
            let new_cfg = run_config_wizard(cfg);
            *cfg = new_cfg;
            *model = cfg.model.clone().unwrap_or_else(default_model);
            app.set_model_config(model.clone());
            app.status = agent_ui::StatusType::Success;
        }
        "/status" | "/session" => {
            if cmd == "/status" {
                app.status = agent_ui::StatusType::Success;
            } else {
                app.status = agent_ui::StatusType::Success;
            }
        }
        "/tools" => {
            app.status = agent_ui::StatusType::Success;
        }
        "/clear" => {
            app.clear_messages();
        }
        "/compact" | "/undo" | "/retry" | "/diff" => {
            app.status = agent_ui::StatusType::Success;
        }
        "/mcp" => {
            app.status = agent_ui::StatusType::Success;
        }
        "/skills" => {
            app.status = agent_ui::StatusType::Success;
        }
        "/plugins" => {
            app.status = agent_ui::StatusType::Success;
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
