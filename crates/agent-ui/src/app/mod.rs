 pub mod state;
 
 use agent_config::ModelConfig;
 use crossterm::event::KeyCode;
 
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
                 provider: agent_config::ProviderKind::Mock,
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
 
     pub fn handle_event(&mut self, event: KeyCode) -> Option<AppEvent> {
         match &self.current_state {
             AppState::Banner => handle_banner_event(self, event),
             AppState::Chat => handle_chat_event(self, event),
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
             self.tool_calls.push(ToolCallEntry {
                 name: name.to_string(),
                 args: args.to_string(),
                 status: ToolStatus::Running,
                 output: Some(args.to_string()),
             });
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
 
     pub fn render(&self, f: &mut ratatui::prelude::Frame, layout: ratatui::prelude::Layout) {
         render::render_app(f, layout, self)
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
 
 fn handle_banner_event(app: &mut AgentApp, event: KeyCode) -> Option<AppEvent> {
     match event {
         KeyCode::Enter => {
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
         KeyCode::Char('/') => {
             app.autocomplete = vec![
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
 
 fn handle_chat_event(app: &mut AgentApp, event: KeyCode) -> Option<AppEvent> {
     match event {
         KeyCode::Enter => {
             if !app.input.trim().is_empty() {
                 let input = app.input.clone();
                 app.input.clear();
                 Some(AppEvent::SendMessage(input))
             } else {
                 None
             }
         }
         KeyCode::Backspace => {
             app.input.pop();
             None
         }
         KeyCode::Char(c) => {
             app.input.push(c);
             None
         }
         KeyCode::Esc => {
             if !app.input.is_empty() {
                 app.input.clear();
                 None
             } else {
                 app.current_state = AppState::Banner;
                 Some(AppEvent::StateChanged(AppState::Banner))
             }
         }
         KeyCode::Up => {
             if !app.autocomplete.is_empty() {
                 Some(AppEvent::SlashCommand(
                     app.autocomplete.first().unwrap().clone()
                 ))
             } else {
                 None
             }
         }
         KeyCode::Tab => {
             if !app.autocomplete.is_empty() {
                 app.input.push_str(&app.autocomplete[0]);
                 app.autocomplete.clear();
             }
             None
         }
         _ => None,
     }
 }
 
 mod render;
