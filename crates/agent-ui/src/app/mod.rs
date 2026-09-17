use crate::completion::{CompletionEngine, replace_token};
use crate::editor::InputEditor;
use crate::input::SlashCommandPalette;
use agent_config::ModelConfig;
use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};

pub mod render;
pub mod state;

pub use state::{AppEvent, AppState, MessageRole, StatusType, ToolStatus, provider_name};

#[derive(Debug, Clone)]
pub struct MessageEntry {
    pub role: MessageRole,
    pub content: String,
    pub tool_call: Option<String>,
    pub finished: bool,
}

#[derive(Debug, Clone)]
pub struct ToolCallEntry {
    pub name: String,
    pub args: String,
    pub status: ToolStatus,
    pub output: Option<String>,
}

#[derive(Debug, Clone)]
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

pub struct AgentApp {
    pub title: String,
    pub version: String,
    pub messages: Vec<MessageEntry>,
    pub tool_calls: Vec<ToolCallEntry>,
    pub editor: InputEditor,
    pub palette: SlashCommandPalette,
    pub completion: CompletionEngine,
    pub status: StatusType,
    pub compacted_count: usize,
    pub current_state: AppState,
    pub model_config: ModelConfig,
    pub scroll_from_bottom: usize,
    pub stick_to_bottom: bool,
}

impl AgentApp {
    pub fn new(title: &str, version: &str) -> Self {
        Self {
            title: title.to_string(),
            version: version.to_string(),
            messages: Vec::new(),
            tool_calls: Vec::new(),
            editor: InputEditor::new(),
            palette: SlashCommandPalette::new(),
            completion: CompletionEngine::new(),
            status: StatusType::Ready,
            compacted_count: 0,
            current_state: AppState::Banner,
            model_config: ModelConfig {
                provider: agent_config::ProviderKind::Mock,
                model: "mock-1".into(),
                base_url: None,
                api_key_env: None,
                temperature: None,
                max_tokens: None,
            },
            scroll_from_bottom: 0,
            stick_to_bottom: true,
        }
    }

    pub fn set_status(&mut self, status: StatusType) {
        self.status = status;
    }

    pub fn set_model_config(&mut self, model: ModelConfig) {
        self.model_config = model;
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

    pub fn handle_event(&mut self, key: &KeyEvent) -> Option<AppEvent> {
        match &self.current_state {
            AppState::Banner => handle_banner_event(self, key),
            AppState::Chat => handle_chat_event(self, key),
            AppState::ModelWizard | AppState::ProviderWizard | AppState::ConfigMenu => {
                match key.code {
                    KeyCode::Char('q') | KeyCode::Esc => {
                        self.current_state = AppState::Chat;
                        Some(AppEvent::StateChanged(AppState::Chat))
                    }
                    _ => None,
                }
            }
        }
    }

    pub fn push_assistant(&mut self, text: &str) {
        self.messages.push(MessageEntry {
            role: MessageRole::Assistant,
            content: text.to_string(),
            tool_call: None,
            finished: true,
        });
        self.scroll_to_bottom();
    }

    pub fn push_user(&mut self, text: &str) {
        self.messages.push(MessageEntry {
            role: MessageRole::User,
            content: text.to_string(),
            tool_call: None,
            finished: true,
        });
        self.scroll_to_bottom();
    }

    pub fn push_tool_call(&mut self, name: &str, args: &str) {
        let trimmed = textwrap::fill(args, 120);

        if let Some(tool) = self.tool_calls.last_mut() {
            tool.output = Some(args.to_string());
        } else {
            self.tool_calls.push(ToolCallEntry {
                name: name.to_string(),
                args: args.to_string(),
                status: ToolStatus::Running,
                output: Some(args.to_string()),
            });
        }

        self.messages.push(MessageEntry {
            role: MessageRole::Tool,
            content: format!("{name}: {trimmed}"),
            tool_call: Some(name.to_string()),
            finished: true,
        });
        self.scroll_to_bottom();
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
        self.scroll_to_bottom();
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
        self.scroll_to_bottom();
    }

    pub fn input(&self) -> &str {
        &self.editor.content
    }

    pub fn render(&self, f: &mut ratatui::prelude::Frame, layout: ratatui::prelude::Layout) {
        render::render_app(f, layout, self)
    }

    pub fn scroll_to_bottom(&mut self) {
        self.stick_to_bottom = true;
        self.scroll_from_bottom = 0;
    }

    pub fn set_input(&mut self, text: &str) {
        self.editor.set_content(text);
        self.refresh_completion();
    }

    pub fn clear_input(&mut self) {
        self.editor.set_content("");
        self.completion.reset();
    }

    fn refresh_completion(&mut self) {
        self.completion
            .update(&self.editor.content, self.editor.cursor, &self.palette);
    }

    pub fn build_status_text(&self) -> String {
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
                    format!("Error: {e}")
                }
            }
            StatusType::Tool(name) => name.clone(),
        }
    }
}

fn handle_banner_event(app: &mut AgentApp, key: &KeyEvent) -> Option<AppEvent> {
    match key.code {
        KeyCode::Enter | KeyCode::Char(' ') => {
            app.current_state = AppState::Chat;
            Some(AppEvent::StateChanged(AppState::Chat))
        }
        KeyCode::Char('q') | KeyCode::Esc => Some(AppEvent::Shutdown),
        KeyCode::Char('m') | KeyCode::Char('M') => {
            app.current_state = AppState::ModelWizard;
            Some(AppEvent::StateChanged(AppState::ModelWizard))
        }
        KeyCode::Char('p') | KeyCode::Char('P') => {
            app.current_state = AppState::ProviderWizard;
            Some(AppEvent::StateChanged(AppState::ProviderWizard))
        }
        KeyCode::Char('c') | KeyCode::Char('C') => {
            app.current_state = AppState::ConfigMenu;
            Some(AppEvent::StateChanged(AppState::ConfigMenu))
        }
        _ => None,
    }
}

fn handle_chat_event(app: &mut AgentApp, key: &KeyEvent) -> Option<AppEvent> {
    let ctrl = key.modifiers.contains(KeyModifiers::CONTROL);
    let shift = key.modifiers.contains(KeyModifiers::SHIFT);

    match key.code {
        KeyCode::Char('c') if ctrl => None,
        KeyCode::Char('u') if ctrl => {
            app.editor.clear_line();
            app.completion.reset();
            None
        }
        KeyCode::Char('w') if ctrl => {
            app.editor.delete_word();
            app.refresh_completion();
            None
        }
        KeyCode::Char('l') | KeyCode::Char('d') if ctrl => None,
        KeyCode::Enter | KeyCode::Char('\n') if !shift && !ctrl => {
            handle_submit(app)
        }
        KeyCode::Enter | KeyCode::Char('\n') if shift => {
            app.editor.insert_char('\n');
            app.completion.reset();
            None
        }
        KeyCode::Backspace => {
            app.editor.backspace();
            app.refresh_completion();
            None
        }
        KeyCode::Delete => {
            app.editor.delete();
            app.refresh_completion();
            None
        }
        KeyCode::Left => {
            app.editor.move_left();
            app.refresh_completion();
            None
        }
        KeyCode::Right => {
            app.editor.move_right();
            app.refresh_completion();
            None
        }
        KeyCode::Home => {
            app.editor.move_home();
            app.refresh_completion();
            None
        }
        KeyCode::End => {
            app.editor.move_end();
            app.refresh_completion();
            None
        }
        KeyCode::Tab => {
            if app.completion.visible() {
                apply_selected_completion(app);
            }
            None
        }
        KeyCode::Up => {
            if app.completion.visible() {
                app.completion.select_prev();
            } else {
                app.editor.prev_history();
            }
            None
        }
        KeyCode::Down => {
            if app.completion.visible() {
                app.completion.select_next();
            } else {
                app.editor.next_history();
            }
            None
        }
        KeyCode::PageUp => {
            app.scroll_from_bottom = app.scroll_from_bottom.saturating_add(15);
            app.stick_to_bottom = false;
            None
        }
        KeyCode::PageDown => {
            app.scroll_from_bottom = app.scroll_from_bottom.saturating_sub(15);
            app.stick_to_bottom = app.scroll_from_bottom == 0;
            None
        }
        KeyCode::Char(c) => {
            app.editor.insert_char(c);
            app.refresh_completion();
            None
        }
        KeyCode::Esc => {
            if app.completion.visible() {
                app.completion.reset();
            } else if !app.editor.content.is_empty() {
                app.editor.set_content("");
                app.completion.reset();
            } else {
                app.current_state = AppState::Banner;
                return Some(AppEvent::StateChanged(AppState::Banner));
            }
            None
        }
        _ => None,
    }
}

fn handle_submit(app: &mut AgentApp) -> Option<AppEvent> {
    if app.completion.visible() {
        if app.completion.kind == crate::completion::CompleteKind::Slash {
            let item = app.completion.current().cloned();
            if let Some(item) = item {
                app.completion.reset();
                app.editor.set_content("");
                return Some(AppEvent::SlashCommand(
                    item.value.trim_start_matches('/').to_string(),
                ));
            }
        }
        apply_selected_completion(app);
        return None;
    }

    let input = app.editor.submit();
    if input.is_empty() {
        return None;
    }
    if input.starts_with('/') {
        Some(AppEvent::SlashCommand(
            input.trim_start_matches('/').to_string(),
        ))
    } else {
        Some(AppEvent::SendMessage(input))
    }
}

fn apply_selected_completion(app: &mut AgentApp) {
    let Some(item) = app.completion.current().cloned() else {
        return;
    };

    match app.completion.kind {
        crate::completion::CompleteKind::Slash => {
            app.editor.set_content(&item.value);
            app.completion.reset();
        }
        crate::completion::CompleteKind::File => {
            replace_token(&mut app.editor.content, app.editor.cursor, &item.value);
            app.editor.cursor = app.editor.content.len();
            app.completion.reset();
        }
        crate::completion::CompleteKind::None => {}
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn key(code: KeyCode) -> KeyEvent {
        KeyEvent::new(code, KeyModifiers::NONE)
    }

    fn chat_app() -> AgentApp {
        let mut app = AgentApp::new("test", "0.0.0");
        app.set_state(AppState::Chat);
        app
    }

    #[test]
    fn send_message_on_enter() {
        let mut app = chat_app();
        for c in "hello".chars() {
            app.handle_event(&key(KeyCode::Char(c)));
        }
        let ev = app.handle_event(&key(KeyCode::Enter));
        assert!(matches!(ev, Some(AppEvent::SendMessage(s)) if s == "hello"));
        assert!(app.input().is_empty());
    }

    #[test]
    fn slash_command_on_enter() {
        let mut app = chat_app();
        for c in "/help".chars() {
            app.handle_event(&key(KeyCode::Char(c)));
        }
        let ev = app.handle_event(&key(KeyCode::Enter));
        assert!(matches!(ev, Some(AppEvent::SlashCommand(c)) if c == "help"));
    }

    #[test]
    fn tab_completes_slash_command() {
        let mut app = chat_app();
        app.handle_event(&key(KeyCode::Char('/')));
        app.handle_event(&key(KeyCode::Char('m')));
        assert!(app.completion.visible());
        app.handle_event(&key(KeyCode::Tab));
        assert!(app.input().starts_with("/model"));
    }

    #[test]
    fn arrows_navigate_history() {
        let mut app = chat_app();
        for c in "echo one".chars() {
            app.handle_event(&key(KeyCode::Char(c)));
        }
        app.handle_event(&key(KeyCode::Enter));
        for c in "echo two".chars() {
            app.handle_event(&key(KeyCode::Char(c)));
        }
        app.handle_event(&key(KeyCode::Enter));
        app.handle_event(&key(KeyCode::Up));
        assert_eq!(app.input(), "echo two");
        app.handle_event(&key(KeyCode::Up));
        assert_eq!(app.input(), "echo one");
    }

    #[test]
    fn shift_enter_inserts_newline() {
        let mut app = chat_app();
        app.handle_event(&key(KeyCode::Char('a')));
        let ev = app.handle_event(&KeyEvent::new(KeyCode::Enter, KeyModifiers::SHIFT));
        assert!(ev.is_none());
        assert_eq!(app.input(), "a\n");
    }

    #[test]
    fn esc_clears_then_returns_to_banner() {
        let mut app = chat_app();
        app.handle_event(&key(KeyCode::Char('x')));
        app.handle_event(&key(KeyCode::Esc));
        assert!(app.input().is_empty());
        let ev = app.handle_event(&key(KeyCode::Esc));
        assert!(matches!(ev, Some(AppEvent::StateChanged(AppState::Banner))));
    }

    #[test]
    fn cursor_edit_midline() {
        let mut app = chat_app();
        for c in "abde".chars() {
            app.handle_event(&key(KeyCode::Char(c)));
        }
        app.handle_event(&key(KeyCode::Left));
        app.handle_event(&key(KeyCode::Char('c')));
        assert_eq!(app.input(), "abdce");
    }
}