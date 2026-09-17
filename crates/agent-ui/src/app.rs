use agent_config::{ModelConfig, ProviderKind};
use agent_tools::ToolOutput;
use ratatui::prelude::*;
use ratatui::widgets::Wrap;
use std::io::Result;
use crossterm::event::KeyCode;

#[derive(Debug, Clone, PartialEq)]
pub enum AppState {
    Banner,
    Input,
    ModelWizard,
    ProviderWizard,
    ConfigMenu,
}

impl Default for AppState {
    fn default() -> Self {
        AppState::Banner
    }
}

const VS_DARK_BG: Color = Color::Rgb(30, 30, 30);
const VS_DARK_ACCENT: Color = Color::Rgb(86, 156, 214);
const VS_DARK_TEXT: Color = Color::White;

pub enum AppEvent {
    None,
    Shutdown,
    SendMessage(String),
    SlashCommand(String),
    ToolStarted(String),
    ToolFinished(ToolOutput),
    AssistantDelta(String),
    UserInput(String),
    StateChanged(AppState),
}

pub enum AppCommand {
    None,
    Exit,
    Compact,
    Retry,
    Undo,
    Clear,
    ShowDiff,
}

pub struct AgentApp {
    pub title: String,
    pub version: String,
    pub messages: Vec<MessageEntry>,
    pub input: String,
    pub autocomplete: Vec<String>,
    pub spinner_active: bool,
    pub spinner_text: String,
    pub last_tool: Option<String>,
    pub last_tool_output: Option<String>,
    pub status: String,
    pub compacted_count: usize,
    pub current_state: AppState,
}

pub struct MessageEntry {
    pub role: String,
    pub content: String,
    pub tool_call: Option<String>,
    pub finished: bool,
}

impl AgentApp {
    pub fn new(title: &str, version: &str) -> Self {
        Self {
            title: title.to_string(),
            version: version.to_string(),
            messages: Vec::new(),
            input: String::new(),
            autocomplete: Vec::new(),
            spinner_active: false,
            spinner_text: String::new(),
            last_tool: None,
            last_tool_output: None,
            status: String::new(),
            compacted_count: 0,
            current_state: AppState::Banner,
        }
    }

    pub fn handle_event(&mut self, event: KeyCode) -> Option<AppEvent> {
        match &self.current_state {
            AppState::Banner => {
                match event {
                    KeyCode::Enter => {
                        self.current_state = AppState::Input;
                        Some(AppEvent::StateChanged(AppState::Input))
                    }
                    KeyCode::Char('q') | KeyCode::Esc => Some(AppEvent::Shutdown),
                    KeyCode::Char('m') => {
                        self.current_state = AppState::ModelWizard;
                        Some(AppEvent::StateChanged(AppState::ModelWizard))
                    }
                    KeyCode::Char('p') => {
                        self.current_state = AppState::ProviderWizard;
                        Some(AppEvent::StateChanged(AppState::ProviderWizard))
                    }
                    KeyCode::Char('c') => {
                        self.current_state = AppState::ConfigMenu;
                        Some(AppEvent::StateChanged(AppState::ConfigMenu))
                    }
                    _ => None,
                }
            }
            AppState::Input => {
                match event {
                    KeyCode::Enter => {
                        if !self.input.trim().is_empty() {
                            let input = self.input.clone();
                            self.input.clear();
                            Some(AppEvent::SendMessage(input))
                        } else {
                            None
                        }
                    }
                    KeyCode::Backspace => {
                        self.input.pop();
                        None
                    }
                    KeyCode::Char(c) => {
                        self.input.push(c);
                        None
                    }
                    KeyCode::Esc => {
                        self.current_state = AppState::Banner;
                        Some(AppEvent::StateChanged(AppState::Banner))
                    }
                    KeyCode::Up => {
                        if self.autocomplete.len() > 0 {
                            Some(AppEvent::SlashCommand(
                                self.autocomplete.first().unwrap().clone()
                            ))
                        } else {
                            None
                        }
                    }
                    _ => None,
                }
            }
            AppState::ModelWizard => {
                match event {
                    KeyCode::Char('q') | KeyCode::Esc => {
                        self.current_state = AppState::Banner;
                        Some(AppEvent::StateChanged(AppState::Banner))
                    }
                    _ => None,
                }
            }
            AppState::ProviderWizard => {
                match event {
                    KeyCode::Char('q') | KeyCode::Esc => {
                        self.current_state = AppState::Banner;
                        Some(AppEvent::StateChanged(AppState::Banner))
                    }
                    _ => None,
                }
            }
            AppState::ConfigMenu => {
                match event {
                    KeyCode::Char('q') | KeyCode::Esc => {
                        self.current_state = AppState::Banner;
                        Some(AppEvent::StateChanged(AppState::Banner))
                    }
                    _ => None,
                }
            }
        }
    }

    pub fn render(&self, f: &mut Frame, layout: Layout) -> Result<()> {
        let chunks = Layout::default()
            .direction(ratatui::layout::Direction::Vertical)
            .constraints([
                Constraint::Length(3),
                Constraint::Min(0),
                Constraint::Length(3),
            ])
            .split(f.area());

        // Header
        let header = ratatui::widgets::Block::default()
            .borders(ratatui::widgets::Borders::ALL)
            .border_type(ratatui::widgets::BorderType::Rounded)
            .title(format!("{}  v{}", self.title, self.version))
            .title_style(Style::default().fg(VS_DARK_ACCENT).bg(VS_DARK_BG))
            .style(Style::default().bg(VS_DARK_BG))
            .border_style(Style::default().fg(VS_DARK_ACCENT).bg(VS_DARK_BG));
        f.render_widget(header, chunks[0]);

        // Content based on state
        match &self.current_state {
            AppState::Banner => {
                let banner_text = format!(
                    "Agent CLI\nVersion: {}\n\n[Enter] - Start chat\n[m] - Model Wizard\n[p] - Provider Wizard\n[c] - Config Menu\n[q] - Quit",
                    self.version
                );
                let block = ratatui::widgets::Block::default()
                    .style(Style::default().fg(VS_DARK_TEXT).bg(VS_DARK_BG));
                let paragraph = ratatui::widgets::Paragraph::new(banner_text)
                    .style(Style::default().fg(VS_DARK_TEXT).bg(VS_DARK_BG))
                    .block(block);
                f.render_widget(paragraph, chunks[1]);
            }
            AppState::Input => {
                let spinner = if self.spinner_active {
                    format!("⠋ {}", self.spinner_text)
                } else if let Some(t) = &self.last_tool {
                    format!("✓ {}", t)
                } else {
                    "idle".to_string()
                };
                let spinner_widget = ratatui::widgets::Paragraph::new(spinner)
                    .style(Style::default().fg(VS_DARK_ACCENT).bg(VS_DARK_BG))
                    .wrap(Wrap { trim: true });
                f.render_widget(spinner_widget, chunks[1]);
            }
            AppState::ModelWizard => {
                let block = ratatui::widgets::Block::default()
                    .style(Style::default().fg(VS_DARK_TEXT).bg(VS_DARK_BG))
                    .title("Model Wizard")
                    .title_style(Style::default().fg(VS_DARK_ACCENT).bg(VS_DARK_BG));
                let paragraph = ratatui::widgets::Paragraph::new(
                    "Select model:\n\n[1] gpt-4\n[2] gpt-3.5-turbo\n[3] claude\n\n[Esc] - Back to Banner"
                ).block(block);
                f.render_widget(paragraph, chunks[1]);
            }
            AppState::ProviderWizard => {
                let block = ratatui::widgets::Block::default()
                    .style(Style::default().fg(VS_DARK_TEXT).bg(VS_DARK_BG))
                    .title("Provider Wizard")
                    .title_style(Style::default().fg(VS_DARK_ACCENT).bg(VS_DARK_BG));
                let paragraph = ratatui::widgets::Paragraph::new(
                    "Select provider:\n\n[1] OpenAI\n[2] Anthropic\n[3] Google\n\n[Esc] - Back to Banner"
                ).block(block);
                f.render_widget(paragraph, chunks[1]);
            }
            AppState::ConfigMenu => {
                let block = ratatui::widgets::Block::default()
                    .style(Style::default().fg(VS_DARK_TEXT).bg(VS_DARK_BG))
                    .title("Config Menu")
                    .title_style(Style::default().fg(VS_DARK_ACCENT).bg(VS_DARK_BG));
                let paragraph = ratatui::widgets::Paragraph::new(
                    "Configuration:\n\n[r] - Reload config\n[s] - Save config\n\n[Esc] - Back to Banner"
                ).block(block);
                f.render_widget(paragraph, chunks[1]);
            }
        }

        // Input line / status
        let status_text = if self.current_state == AppState::Input {
            if self.input.is_empty() {
                format!("> ")
            } else {
                format!("> {}", self.input)
            }
        } else {
            String::new()
        };

        let input_block = ratatui::widgets::Block::default()
            .borders(ratatui::widgets::Borders::ALL)
            .border_type(ratatui::widgets::BorderType::Rounded)
            .style(Style::default().bg(VS_DARK_BG))
            .title("")
            .title_style(Style::default().bg(VS_DARK_BG));

        let input_widget = ratatui::widgets::Paragraph::new(status_text)
            .style(Style::default().fg(VS_DARK_TEXT).bg(VS_DARK_BG))
            .block(input_block);
        f.render_widget(input_widget, chunks[2]);

        Ok(())
    }

    pub fn push_assistant(&mut self, text: &str) {
        self.messages.push(MessageEntry {
            role: "assistant".into(),
            content: text.to_string(),
            tool_call: None,
            finished: true,
        });
    }

    pub fn push_user(&mut self, text: &str) {
        self.messages.push(MessageEntry {
            role: "user".into(),
            content: text.to_string(),
            tool_call: None,
            finished: true,
        });
    }

    pub fn push_tool_call(&mut self, name: &str, args: &str) {
        self.messages.push(MessageEntry {
            role: "tool".into(),
            content: args.to_string(),
            tool_call: Some(name.to_string()),
            finished: true,
        });
    }

    pub fn set_spinner(&mut self, active: bool, text: &str) {
        self.spinner_active = active;
        self.spinner_text = text.to_string();
    }

    pub fn set_state(&mut self, state: AppState) {
        self.current_state = state;
    }

    pub fn append_status(&mut self, text: &str) {
        self.status = text.to_string();
    }

    pub fn append_assistant(&mut self, text: &str) {
        self.messages.push(MessageEntry {
            role: "assistant".into(),
            content: text.to_string(),
            tool_call: None,
            finished: true,
        });
    }

    pub fn set_model_config(&mut self, model: ModelConfig) {
        let _ = model;
    }

    pub fn compact(&mut self) {
        self.compacted_count += 1;
        let keep = 4.min(self.messages.len());
        let drop = self.messages.len() - keep;
        self.messages.drain(0..drop.max(0));
    }

    pub fn undo_last(&mut self) {
        if let Some(last) = self.messages.pop() {
            if last.role == "tool" {
                self.messages.push(last);
            }
        }
    }
}

pub fn render_frame(
    f: &mut Frame,
    app: &mut AgentApp,
    tool_widget: &ToolCallWidget,
) -> Result<()> {
    let area = f.area();
    let chunks = ratatui::layout::Layout::default()
        .direction(ratatui::layout::Direction::Vertical)
        .constraints([
            Constraint::Length(3),
            Constraint::Min(0),
            Constraint::Length(3),
        ])
        .split(area);

    let header = ratatui::widgets::Block::default()
        .borders(ratatui::widgets::Borders::ALL)
        .border_type(ratatui::widgets::BorderType::Rounded)
        .title(format!(
            "{}  v{}",
            app.title, app.version
        ))
        .title_alignment(ratatui::layout::Alignment::Center);
    f.render_widget(header, chunks[0]);

    let spinner = if app.spinner_active {
        format!("⠋ {}", app.spinner_text)
    } else if let Some(t) = &app.last_tool {
        format!("✓ {}", t)
    } else {
        "idle".to_string()
    };

    let spinner_widget = ratatui::widgets::Paragraph::new(spinner)
        .wrap(Wrap { trim: true });
    f.render_widget(spinner_widget, chunks[1]);

    let status = if app.input.is_empty() {
        format!(
            "{}  [messages: {}] {}",
            app.status,
            app.messages.len(),
            tool_widget.label()
        )
    } else {
        app.input.clone()
    };
    let status_widget = ratatui::widgets::Paragraph::new(status).wrap(Wrap { trim: true });
    f.render_widget(status_widget, chunks[2]);
    Ok(())
}

pub struct ToolCallWidget {
    pub tool_name: Option<String>,
    pub tool_output: Option<String>,
}

impl ToolCallWidget {
    pub fn new() -> Self {
        Self {
            tool_name: None,
            tool_output: None,
        }
    }

    pub fn update(&mut self, name: &str, output: &str) {
        self.tool_name = Some(name.to_string());
        self.tool_output = Some(output.to_string());
    }

    pub fn label(&self) -> String {
        match &self.tool_name {
            None => String::from("no tool"),
            Some(n) => format!("tool: {n}"),
        }
    }

    pub fn collapsed_body(&self) -> String {
        self.tool_output.clone().unwrap_or_default()
    }
}

impl Default for ToolCallWidget {
    fn default() -> Self {
        Self::new()
    }
}
