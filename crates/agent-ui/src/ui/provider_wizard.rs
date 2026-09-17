 use agent_config::{ModelConfig, ProviderKind};
 
 pub struct ProviderWizardScreen {
     pub config: ModelConfig,
     pub current_step: WizardStep,
     pub input_value: String,
     pub is_editable: bool,
 }
 
 #[derive(Debug, Clone, PartialEq)]
 pub enum WizardStep {
     ProviderId,
     BaseUrl,
     ApiKeyEnv,
     Check,
 }
 
 impl Default for ProviderWizardScreen {
     fn default() -> Self {
         Self::new()
     }
 }
 
 impl ProviderWizardScreen {
     pub fn new() -> Self {
         Self {
             config: ModelConfig {
                 provider: ProviderKind::Mock,
                 model: "mock-1".into(),
                 base_url: None,
                 api_key_env: None,
                 temperature: None,
                 max_tokens: None,
             },
             current_step: WizardStep::ProviderId,
             input_value: String::new(),
             is_editable: true,
         }
     }
 
     pub fn from_config(config: ModelConfig) -> Self {
         let mut wizard = Self::new();
         wizard.config = config;
         wizard.update_input_from_config();
         wizard
     }
 
     fn update_input_from_config(&mut self) {
         self.input_value = match self.current_step {
             WizardStep::ProviderId => provider_name(&self.config.provider).to_string(),
             WizardStep::BaseUrl => self.config.base_url.clone().unwrap_or_default(),
             WizardStep::ApiKeyEnv => self.config.api_key_env.clone().unwrap_or_default(),
             WizardStep::Check => String::new(),
         };
     }
 
     pub fn step_index(&self) -> usize {
         match self.current_step {
             WizardStep::ProviderId => 0,
             WizardStep::BaseUrl => 1,
             WizardStep::ApiKeyEnv => 2,
             WizardStep::Check => 3,
         }
     }
 
     pub fn select_next_step(&mut self) {
         self.current_step = match self.current_step {
             WizardStep::ProviderId => WizardStep::BaseUrl,
             WizardStep::BaseUrl => WizardStep::ApiKeyEnv,
             WizardStep::ApiKeyEnv => WizardStep::Check,
             WizardStep::Check => WizardStep::Check,
         };
         self.update_input_from_config();
     }
 
     pub fn select_previous_step(&mut self) {
         self.current_step = match self.current_step {
             WizardStep::ProviderId => WizardStep::Check,
             WizardStep::BaseUrl => WizardStep::ProviderId,
             WizardStep::ApiKeyEnv => WizardStep::BaseUrl,
             WizardStep::Check => WizardStep::ApiKeyEnv,
         };
         self.update_input_from_config();
     }
 
     pub fn get_provider_index(&self) -> usize {
         self.provider_options()
             .iter()
             .position(|p| *p == provider_name(&self.config.provider))
             .unwrap_or(0)
     }
 
     pub fn set_provider_index(&mut self, index: usize) {
         if index < self.provider_options().len() {
             let provider = self.provider_options()[index];
             self.config.provider = match provider {
                 "openai" => ProviderKind::OpenAi,
                 "anthropic" => ProviderKind::Anthropic,
                 "google" => ProviderKind::Google,
                 "mock" => ProviderKind::Mock,
                 "custom / openai-compatible" => ProviderKind::OpenAiCompatible,
                 _ => ProviderKind::OpenAi,
             };
             if self.current_step == WizardStep::ProviderId {
                 self.input_value = provider_name(&self.config.provider).to_string();
             }
         }
     }
 
     pub fn next_provider(&mut self) {
         let new_index = (self.get_provider_index() + 1) % self.provider_options().len();
         self.set_provider_index(new_index);
     }
 
     pub fn previous_provider(&mut self) {
         let current = self.get_provider_index();
         let new_index = if current == 0 {
             self.provider_options().len() - 1
         } else {
             current - 1
         };
         self.set_provider_index(new_index);
     }
 
     fn provider_options(&self) -> Vec<&'static str> {
         vec!["openai", "anthropic", "google", "mock", "custom / openai-compatible"]
     }
 
     pub fn next_step(&mut self) {
         match self.current_step {
             WizardStep::BaseUrl => {
                 self.config.base_url = if self.input_value.is_empty() {
                     None
                 } else {
                     Some(self.input_value.clone())
                 };
             }
             WizardStep::ApiKeyEnv => {
                 self.config.api_key_env = if self.input_value.is_empty() {
                     None
                 } else {
                     Some(self.input_value.clone())
                 };
             }
             _ => {}
         }
         self.select_next_step();
     }
 
     pub fn previous_step(&mut self) {
         self.select_previous_step();
     }
 
     pub fn set_input_value(&mut self, value: String) {
         self.input_value = value;
     }
 
     pub fn input_value(&self) -> &str {
         &self.input_value
     }
 
     pub fn validate_field(&self) -> ValidationResult {
         match self.current_step {
             WizardStep::ProviderId => {
                 if self.input_value.is_empty() {
                     ValidationResult::Invalid("Provider ID cannot be empty".to_string())
                 } else {
                     ValidationResult::Valid
                 }
             }
             WizardStep::BaseUrl => {
                 if !self.input_value.is_empty() && !self.is_valid_url(&self.input_value) {
                     ValidationResult::Invalid("Invalid Base URL format".to_string())
                 } else {
                     ValidationResult::Valid
                 }
             }
             WizardStep::ApiKeyEnv | WizardStep::Check => ValidationResult::Valid,
         }
     }
 
     pub fn is_valid_url(&self, url: &str) -> bool {
         url.starts_with("http://") || url.starts_with("https://")
     }
 
     pub fn can_check(&self) -> bool {
         let base_url = self.config.base_url.as_ref().map(|s| s.as_str()).unwrap_or("");
         let api_key_env = self.config.api_key_env.as_ref().map(|s| s.as_str()).unwrap_or("");
         !base_url.is_empty() && !api_key_env.is_empty()
     }
 
     pub fn check_connection(&self) -> CheckResult {
         CheckResult::Success
     }
 
     pub fn build_config(&self) -> ModelConfig {
         self.config.clone()
     }
 
     pub fn render_wizard(&self, frame: &mut ratatui::Frame, area: ratatui::layout::Rect) {
         super::provider_wizard_render::render_screen(self, frame, area)
     }
 }
 
 #[derive(Debug, Clone, PartialEq)]
 pub enum ValidationResult {
     Valid,
     Invalid(String),
 }
 
 #[derive(Debug, Clone, PartialEq)]
 pub enum CheckResult {
     Success,
     Failed(String),
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
 
 pub fn run_provider_wizard(_current: Option<&ModelConfig>) -> ModelConfig {
     ProviderWizardScreen::new().build_config()
 }
 
 #[cfg(test)]
 mod tests {
     use super::*;
 
     #[test]
     fn test_provider_wizard_screen_new() {
         let wizard = ProviderWizardScreen::new();
         assert_eq!(wizard.current_step, WizardStep::ProviderId);
         assert_eq!(wizard.config.provider, ProviderKind::Mock);
     }
 
     #[test]
     fn test_step_index() {
         let wizard = ProviderWizardScreen::new();
         assert_eq!(wizard.step_index(), 0);
     }
 
     #[test]
     fn test_select_next_step() {
         let mut wizard = ProviderWizardScreen::new();
         assert_eq!(wizard.current_step, WizardStep::ProviderId);
         wizard.select_next_step();
         assert_eq!(wizard.current_step, WizardStep::BaseUrl);
     }
 
     #[test]
     fn test_select_previous_step() {
         let mut wizard = ProviderWizardScreen::new();
         wizard.select_previous_step();
         assert_eq!(wizard.current_step, WizardStep::Check);
     }
 
     #[test]
     fn test_previous_provider() {
         let mut wizard = ProviderWizardScreen::new();
         wizard.update_input_from_config();
         wizard.previous_provider();
         assert_eq!(wizard.config.provider, ProviderKind::Google);
     }
 
     #[test]
     fn test_set_input_value() {
         let mut wizard = ProviderWizardScreen::new();
         wizard.set_input_value("https://api.example.com".to_string());
         assert_eq!(wizard.input_value(), "https://api.example.com");
     }
 
     #[test]
     fn test_validate_field_empty_provider() {
         let mut wizard = ProviderWizardScreen::new();
         wizard.set_input_value(String::new());
         assert_eq!(
             wizard.validate_field(),
             ValidationResult::Invalid("Provider ID cannot be empty".to_string())
         );
     }
 
     #[test]
     fn test_validate_field_valid_provider() {
         let mut wizard = ProviderWizardScreen::new();
         wizard.update_input_from_config();
         assert_eq!(wizard.validate_field(), ValidationResult::Valid);
     }
 
     #[test]
     fn test_validate_field_invalid_url() {
         let mut wizard = ProviderWizardScreen::new();
         wizard.current_step = WizardStep::BaseUrl;
         wizard.set_input_value("invalid-url".to_string());
         assert_eq!(
             wizard.validate_field(),
             ValidationResult::Invalid("Invalid Base URL format".to_string())
         );
     }
 
     #[test]
     fn test_validate_field_valid_url() {
         let mut wizard = ProviderWizardScreen::new();
         wizard.current_step = WizardStep::BaseUrl;
         wizard.set_input_value("https://api.example.com".to_string());
         assert_eq!(wizard.validate_field(), ValidationResult::Valid);
     }
 
     #[test]
     fn test_is_valid_url() {
         let wizard = ProviderWizardScreen::new();
         assert!(wizard.is_valid_url("http://example.com"));
         assert!(wizard.is_valid_url("https://example.com"));
         assert!(!wizard.is_valid_url("example.com"));
         assert!(!wizard.is_valid_url("ftp://example.com"));
     }
 
     #[test]
     fn test_build_config() {
         let mut wizard = ProviderWizardScreen::new();
         wizard.config.model = "gpt-4".to_string();
         wizard.config.provider = ProviderKind::OpenAi;
         wizard.config.base_url = Some("https://api.openai.com".to_string());
         wizard.config.api_key_env = Some("OPENAI_API_KEY".to_string());
         let config = wizard.build_config();
         assert_eq!(config.model, "gpt-4");
         assert_eq!(config.provider, ProviderKind::OpenAi);
         assert_eq!(config.base_url, Some("https://api.openai.com".to_string()));
         assert_eq!(config.api_key_env, Some("OPENAI_API_KEY".to_string()));
     }
 }
