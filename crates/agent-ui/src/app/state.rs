 use agent_config::ProviderKind;
 
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
 pub enum StatusType {
     Ready,
     Thinking(String),
     Success,
     Error(String),
     Tool(String),
 }
 
 impl std::fmt::Display for StatusType {
     fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
         match self {
             StatusType::Ready => write!(f, "Ready"),
             StatusType::Thinking(t) => write!(f, "Thinking... {t}"),
             StatusType::Success => write!(f, "Done"),
             StatusType::Error(e) => write!(f, "Error: {e}"),
             StatusType::Tool(t) => write!(f, "Tool: {t}"),
         }
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
