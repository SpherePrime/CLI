use agent_config::ModelConfig;
use agent_config::ProviderKind;
use ratatui::prelude::*;
use ratatui::widgets::{Paragraph, Wrap};
use ratatui::text::Span;
use crossterm::event::KeyCode;

use crate::theme::Theme;
use crate::ui::components::{render_header, render_empty_state};
use crate::ui::banner;

#[derive(Debug, Clone)]
pub enum AppEvent {
    None,
    Shutdown,
    SendMessage(String),
    SlashCommand(String),
    ToolStarted(String),
    AssistantDelta(String),
    StateChanged(AppState),
    Undo,
    Clear,
    Retry,
    ShowDiff,
}

#[derive(Debug, Clone, PartialEq)]
pub enum AppState {
    Banner,
    Chat,
    ModelWizard,
    ProviderWizard,
    ConfigMenu,
}

impl Default for AppState {
    fn default() -> Self {
        AppState::Banner
    }
}

#[derive(Debug, Clone)]
pub enum MessageRole {
    User,
    Assistant,
    Tool,
    System,
}

impl MessageRole {
    pub fn as_str(&self) -> &'static str {
        match self {
            MessageRole::User => "user",
            MessageRole::Assistant => "assistant",
            MessageRole::Tool => "tool",
            MessageRole::System => "system",
        }
    }
}

#[derive(Debug, Clone)]
pub enum ToolStatus {
    Pending,
    Running,
    Success,
    Error,
}

#[derive(Debug, Clone)]
pub struct ToolCallEntry {
    pub name: String,
    pub args: String,
    pub status: ToolStatus,
    pub output: Option<String>,
}

#[derive(Debug, Clone)]
pub struct MessageEntry {
    pub role: MessageRole,
    pub content: String,
    pub tool_call: Option<String>,
    pub finished: bool,
}

#[derive(Debug, Clone)]
pub enum StatusType {
    Ready,
    Thinking(String),
    Success,
    Error(String),
    Tool(String),
}

pub struct AgentApp {
    pub title: String,
    pub version: String,
    pub messages: Vec<MessageEntry>,
    pub tool_calls: Vec<ToolCallEntry>,
    pub input: String,
    pub autocomplete: Vec<String>,
    pub status: StatusType,
    pub compacted_count: usize,
    pub current_state: AppState,
    pub model_config: ModelConfig,
}

impl AgentApp {
    pub fn new(title: &str, version: &str) -> Self {
        Self {
            title: title.to_string(),
            version: version.to_string(),
            messages: Vec::new(),
            tool_calls: Vec::new(),
            input: String::new(),
            autocomplete: Vec::new(),
            status: StatusType::Ready,
            compacted_count: 0,
            current_state: AppState::Banner,
            model_config: ModelConfig {
                provider: ProviderKind::Mock,
                model: "mock-1".into(),
                base_url: None,
                api_key_env: None,
                temperature: None,
                max_tokens: None,
            },
        }
    }

    pub fn set_status(&mut self, status: StatusType) {
        self.status = status;
    }

    pub fn set_model_config(&mut self, model: ModelConfig) {
        self.model_config = model.clone();
    }

    pub fn set_spinner(&mut self, active: bool, text: &str) {
        if active {
            self.status = StatusType::Thinking(text.to_string());
        } else {
            self.status = StatusType::Success;
        }
    }

    pub fn set_state(&mut self, state: AppState) {
        self.current_state = state;
    }

    pub fn handle_event(&mut self, event: KeyCode) -> Option<AppEvent> {
        match &self.current_state {
            AppState::Banner => {
                match event {
                    KeyCode::Enter => {
                        self.current_state = AppState::Chat;
                        Some(AppEvent::StateChanged(AppState::Chat))
                    }
                    KeyCode::Char('q') | KeyCode::Esc => Some(AppEvent::Shutdown),
                    KeyCode::Char('m') | KeyCode::Char('M') => {
                        self.current_state = AppState::ModelWizard;
                        Some(AppEvent::StateChanged(AppState::ModelWizard))
                    }
                    KeyCode::Char('p') | KeyCode::Char('P') => {
                        self.current_state = AppState::ProviderWizard;
                        Some(AppEvent::StateChanged(AppState::ProviderWizard))
                    }
                    KeyCode::Char('c') | KeyCode::Char('C') => {
                        self.current_state = AppState::ConfigMenu;
                        Some(AppEvent::StateChanged(AppState::ConfigMenu))
                    }
                    KeyCode::Char('/') => {
                        self.autocomplete = vec![
                            "/help".to_string(),
                            "/model".to_string(),
                            "/config".to_string(),
                            "/status".to_string(),
                            "/tools".to_string(),
                            "/exit".to_string(),
                        ];
                        Some(AppEvent::SlashCommand("/help".to_string()))
                    }
                    _ => None,
                }
            }
            AppState::Chat => {
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
                        if !self.input.is_empty() {
                            self.input.clear();
                            None
                        } else {
                            self.current_state = AppState::Banner;
                            Some(AppEvent::StateChanged(AppState::Banner))
                        }
                    }
                    KeyCode::Up => {
                        if !self.autocomplete.is_empty() {
                            Some(AppEvent::SlashCommand(
                                self.autocomplete.first().unwrap().clone()
                            ))
                        } else {
                            None
                        }
                    }
                    KeyCode::Tab => {
                        if !self.autocomplete.is_empty() {
                            self.input.push_str(&self.autocomplete[0]);
                            self.autocomplete.clear();
                        }
                        None
                    }
                    _ => None,
                }
            }
            AppState::ModelWizard | AppState::ProviderWizard | AppState::ConfigMenu => {
                match event {
                    KeyCode::Char('q') | KeyCode::Esc => {
                        self.current_state = AppState::Chat;
                        Some(AppEvent::StateChanged(AppState::Chat))
                    }
                    _ => None,
                }
            }
        }
    }

    pub fn render(&self, f: &mut Frame, _layout: Layout) {
        let area = f.area();
        let theme = Theme::default_theme();
        let _ = _layout; // Suppress unused warning
        
        // Render different states
        match self.current_state {
            AppState::Banner => {
                // Banner state - show welcome screen
                let banner = banner::BannerScreen::new()
                    .with_model(&self.model_config.model)
                    .with_provider(provider_name(&self.model_config.provider))
                    .with_version(&self.version);
                banner.render(f, area);
            }
            AppState::Chat => {
                // Chat state - main interface
                // Layout: header, messages, footer
                let chunks = Layout::default()
                    .direction(Direction::Vertical)
                    .constraints([
                        Constraint::Length(1),  // Header
                        Constraint::Min(0),       // Messages
                        Constraint::Length(1),  // Footer
                    ])
                    .split(area);

                // Render header
                let cwd = std::env::current_dir()
                    .unwrap_or_default()
                    .to_string_lossy()
                    .to_string();
                
                render_header(f, chunks[0], &self.model_config.model, 
                    provider_name(&self.model_config.provider), &cwd, &theme);
                
                // Render messages area or empty state
                if self.messages.is_empty() {
                    render_messages_list(f, chunks[1], &self.messages, &theme);
                } else {
                    // Render message history with improved styling
                    let mut lines: Vec<Line> = vec![Line::from("")];
                    
                    for msg in &self.messages {
                        let role_str = match msg.role {
                            MessageRole::User => "USER",
                            MessageRole::Assistant => "AGENT",
                            MessageRole::Tool => "TOOL",
                            MessageRole::System => "RESULT",
                        };
                        
                        let color = match msg.role {
                            MessageRole::User => theme.message_user,
                            MessageRole::Assistant => theme.message_assistant,
                            MessageRole::Tool => theme.message_tool,
                            MessageRole::System => theme.success,
                        };
                        
                        let icon = match msg.role {
                            MessageRole::User => "❯",
                            MessageRole::Assistant => "●",
                            MessageRole::Tool => "→",
                            MessageRole::System => "✓",
                        };
                        
                        let preview = if msg.content.len() > 100 {
                            format!("{}...", &msg.content[..97])
                        } else {
                            msg.content.clone()
                        };
                        
                        lines.push(Line::from(vec![
                            Span::styled(" ", Style::default()),
                            Span::styled(icon, Style::default().fg(color)),
                            Span::styled(" ", Style::default()),
                            Span::styled(role_str, Style::default().fg(color).add_modifier(Modifier::BOLD)),
                            Span::styled(" ", Style::default()),
                            Span::styled(preview, Style::default().fg(theme.foreground)),
                        ]));
                    }
                    
                    let messages_para = Paragraph::new(lines)
                        .style(Style::default().fg(theme.foreground))
                        .wrap(Wrap { trim: true });
                    
                    f.render_widget(messages_para, chunks[1]);
                }
                
                // Render footer with prompt
                let status_text = self.build_status_text();
                
                let footer = Paragraph::new(Line::from(vec![
                    Span::styled("❯ ", Style::default().fg(theme.accent).add_modifier(Modifier::BOLD)),
                    Span::styled(&self.input, Style::default().fg(theme.foreground)),
                    Span::styled(" ", Style::default()),
                    Span::styled(&status_text, Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM)),
                ]))
                .style(Style::default().fg(theme.foreground))
                .wrap(Wrap { trim: true });
                
                f.render_widget(footer, chunks[2]);
            }
            AppState::ModelWizard | AppState::ProviderWizard | AppState::ConfigMenu => {
                // Wizard states - use full area
                let status_text = self.build_status_text();
                
                let status = Paragraph::new(Line::from(vec![
                    Span::styled("❯ ", Style::default().fg(theme.accent).add_modifier(Modifier::BOLD)),
                    Span::styled(&self.input, Style::default().fg(theme.foreground)),
                    Span::styled(" ", Style::default()),
                    Span::styled(&status_text, Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM)),
                ]))
                .style(Style::default().fg(theme.foreground))
                .wrap(Wrap { trim: true });
                
                f.render_widget(status, area);
            }
        }
    }

    fn build_status_text(&self) -> String {
        match &self.status {
            StatusType::Ready => "Ready".to_string(),
            StatusType::Thinking(t) => {
                if t.is_empty() {
                    "Thinking...".to_string()
                } else {
                    t.clone()
                }
            }
            StatusType::Success => "Done".to_string(),
            StatusType::Error(e) => {
                if e.is_empty() {
                    "Error".to_string()
                } else {
                    format!("Error: {}", e)
                }
            }
            StatusType::Tool(name) => name.clone(),
        }
    }

    pub fn push_assistant(&mut self, text: &str) {
        self.messages.push(MessageEntry {
            role: MessageRole::Assistant,
            content: text.to_string(),
            tool_call: None,
            finished: true,
        });
    }

    pub fn push_user(&mut self, text: &str) {
        self.messages.push(MessageEntry {
            role: MessageRole::User,
            content: text.to_string(),
            tool_call: None,
            finished: true,
        });
    }

    pub fn push_tool_call(&mut self, name: &str, args: &str) {
        if let Some(tool) = self.tool_calls.last_mut() {
            tool.output = Some(args.to_string());
        } else {
            let tool = ToolCallEntry {
                name: name.to_string(),
                args: args.to_string(),
                status: ToolStatus::Running,
                output: Some(args.to_string()),
            };
            self.tool_calls.push(tool);
        }
        
        self.messages.push(MessageEntry {
            role: MessageRole::Tool,
            content: args.to_string(),
            tool_call: Some(name.to_string()),
            finished: true,
        });
    }

    pub fn append_status(&mut self, text: &str) {
        self.status = match self.status {
            StatusType::Thinking(_) => StatusType::Thinking(text.to_string()),
            _ => StatusType::Success,
        };
    }

    pub fn finish_tool(&mut self, output: Option<String>) {
        if let Some(tool) = self.tool_calls.last_mut() {
            tool.status = ToolStatus::Success;
            tool.output = output;
        }
    }

    pub fn error(&mut self, msg: &str) {
        self.status = StatusType::Error(msg.to_string());
    }

    pub fn compact(&mut self) {
        self.compacted_count += 1;
        let keep = 8.min(self.messages.len());
        let drop = self.messages.len() - keep;
        self.messages.drain(0..drop.max(0));
    }

    pub fn undo_last(&mut self) {
        if let Some(last) = self.messages.pop() {
            if matches!(last.role, MessageRole::Tool) {
                self.messages.push(last);
            }
        }
    }

    pub fn clear_messages(&mut self) {
        self.messages.clear();
        self.tool_calls.clear();
    }
}

fn render_messages_list(f: &mut Frame, area: Rect, _messages: &[MessageEntry], theme: &Theme) {
    // Empty state message
    let empty_lines: Vec<Line> = vec![
        Line::from(""),
        Line::from(vec![
            Span::styled("Ready to code.", Style::default().fg(theme.foreground).add_modifier(Modifier::BOLD)),
        ]),
        Line::from(""),
        Line::from(vec![
            Span::styled("Ask me to inspect, edit, refactor, test,", Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM)),
        ]),
        Line::from(vec![
            Span::styled("debug, or explain your code.", Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM)),
        ]),
        Line::from(""),
        Line::from(vec![
            Span::styled("Try: \"inspect this project for issues\"", Style::default().fg(theme.accent)),
        ]),
        Line::from(""),
    ];
    
    let empty_para = Paragraph::new(empty_lines)
        .style(Style::default().fg(theme.foreground))
        .wrap(Wrap { trim: true });
    
    f.render_widget(empty_para, area);
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
            Some(n) => format!("tool: {}", n),
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

pub fn provider_name(p: &ProviderKind) -> &'static str {
    match p {
        ProviderKind::OpenAi => "openai",
        ProviderKind::Anthropic => "anthropic",
        ProviderKind::Google => "google",
        ProviderKind::OpenAiCompatible => "openai-compatible",
        ProviderKind::Custom => "custom",
        ProviderKind::Mock => "mock",
    }
}

impl std::fmt::Display for StatusType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            StatusType::Ready => write!(f, "Ready"),
            StatusType::Thinking(t) => write!(f, "Thinking... {}", t),
            StatusType::Success => write!(f, "Done"),
            StatusType::Error(e) => write!(f, "Error: {}", e),
            StatusType::Tool(t) => write!(f, "Tool: {}", t),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_app_state_default() {
        let app = AgentApp::new("Test", "1.0");
        assert!(matches!(app.current_state, AppState::Banner));
    }

    #[test]
    fn test_set_model_config() {
        let mut app = AgentApp::new("Test", "1.0");
        let config = ModelConfig {
            provider: ProviderKind::OpenAi,
            model: "gpt-4".into(),
            base_url: None,
            api_key_env: None,
            temperature: None,
            max_tokens: None,
        };
        app.set_model_config(config);
        assert_eq!(app.model_config.model, "gpt-4");
    }

    #[test]
    fn test_push_messages() {
        let mut app = AgentApp::new("Test", "1.0");
        app.push_user("Hello");
        app.push_assistant("Hi there!");
        assert_eq!(app.messages.len(), 2);
        assert!(matches!(app.messages[0].role, MessageRole::User));
        assert!(matches!(app.messages[1].role, MessageRole::Assistant));
    }
}