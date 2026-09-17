use agent_tools::ToolOutput;
use ratatui::prelude::*;
use ratatui::widgets::Wrap;
use std::io::Result;

pub enum AppEvent {
    None,
    Shutdown,
    SendMessage(String),
    SlashCommand(String),
    ToolStarted(String),
    ToolFinished(ToolOutput),
    AssistantDelta(String),
    UserInput(String),
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
        }
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
