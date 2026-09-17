use agent_config::{ModelConfig, ProviderKind};
use ratatui::layout::{Constraint, Direction, Layout};
use ratatui::style::{Color, Modifier, Style};
use ratatui::widgets::{Block, BorderType, List, ListItem, Paragraph};
use ratatui::Frame;

use super::Component;

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
                if !self.input_value.is_empty() {
                    self.config.base_url = Some(self.input_value.clone());
                } else {
                    self.config.base_url = None;
                }
            }
            WizardStep::ApiKeyEnv => {
                if !self.input_value.is_empty() {
                    self.config.api_key_env = Some(self.input_value.clone());
                } else {
                    self.config.api_key_env = None;
                }
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
            WizardStep::ApiKeyEnv => {
                ValidationResult::Valid
            }
            WizardStep::Check => {
                ValidationResult::Valid
            }
        }
    }

    fn is_valid_url(&self, url: &str) -> bool {
        url.starts_with("http://") || url.starts_with("https://")
    }

    pub fn can_check(&self) -> bool {
        !self.config.base_url.as_ref().map(|s| s.as_str()).unwrap_or("").is_empty()
            && !self.config.api_key_env.as_ref().map(|s| s.as_str()).unwrap_or("").is_empty()
    }

    pub fn check_connection(&self) -> CheckResult {
        CheckResult::Success
    }

    pub fn build_config(&self) -> ModelConfig {
        self.config.clone()
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

impl Component for ProviderWizardScreen {
    fn render(&self, frame: &mut Frame, area: ratatui::layout::Rect) -> ratatui::layout::Rect {
        let main_block = Block::bordered()
            .title("Provider Wizard - Configure your provider")
            .border_style(Style::default().fg(Color::Rgb(79, 193, 255)))
            .border_type(BorderType::Rounded);

        let inner_area = main_block.inner(area);

        frame.render_widget(main_block, area);

        let chunks = Layout::default()
            .direction(Direction::Vertical)
            .margin(2)
            .constraints([
                Constraint::Length(20),
                Constraint::Min(0),
                Constraint::Length(3),
            ])
            .split(inner_area);

        let form_area = chunks[0];
        let preview_area = chunks[1];
        let status_area = chunks[2];

        self.render_form(frame, form_area);
        self.render_preview(frame, preview_area);
        self.render_status(frame, status_area);

        inner_area
    }
}

impl ProviderWizardScreen {
    fn step_title(&self) -> &'static str {
        match self.current_step {
            WizardStep::ProviderId => "Step 1: Provider ID",
            WizardStep::BaseUrl => "Step 2: Base URL",
            WizardStep::ApiKeyEnv => "Step 3: API Key Env",
            WizardStep::Check => "Step 4: Check",
        }
    }

    fn render_form(&self, frame: &mut Frame, area: ratatui::layout::Rect) {
        let block = Block::bordered()
            .title(self.step_title())
            .border_type(BorderType::Rounded)
            .style(Style::default().fg(Color::Rgb(213, 213, 213)));

        let inner = block.inner(area);

        frame.render_widget(block, area);

        let step_label = Paragraph::new(self.get_step_label())
            .style(Style::default().fg(Color::Rgb(156, 163, 175)));

        let input_paragraph = Paragraph::new(self.input_value.as_str())
            .style(Style::default().fg(Color::Rgb(118, 158, 242)).add_modifier(Modifier::BOLD));

        frame.render_widget(step_label, inner);
    }

    fn get_step_label(&self) -> String {
        match self.current_step {
            WizardStep::ProviderId => {
                let selected = provider_name(&self.config.provider);
                format!("Provider: {} (Tab: change provider)", selected)
            }
            WizardStep::BaseUrl => "Base URL:".to_string(),
            WizardStep::ApiKeyEnv => "API Key Env:".to_string(),
            WizardStep::Check => "Connection Check:".to_string(),
        }
    }

    fn render_preview(&self, frame: &mut Frame, area: ratatui::layout::Rect) {
        let block = Block::bordered()
            .title("Preview")
            .border_type(BorderType::Rounded)
            .style(Style::default().fg(Color::Rgb(156, 163, 175)).add_modifier(Modifier::ITALIC));

        let inner = block.inner(area);

        let mut lines = vec![
            format!("Provider: {}", provider_name(&self.config.provider)),
            format!("Model: {}", if self.config.model.is_empty() { "-" } else { &self.config.model }),
        ];

        if let Some(ref url) = self.config.base_url {
            lines.push(format!("Base URL: {}", url));
        }

        if let Some(ref key) = self.config.api_key_env {
            if !key.is_empty() {
                lines.push(format!("API Key Env: {}", key));
            }
        }

        let list_items: Vec<ListItem> = lines
            .iter()
            .map(|l| {
                ListItem::new(l.as_str()).style(Style::default().fg(Color::Rgb(203, 213, 224)))
            })
            .collect();

        let list = List::new(list_items).style(Style::default().fg(Color::Rgb(203, 213, 224)));

        frame.render_widget(block, area);
        frame.render_widget(list, inner);
    }

    fn render_status(&self, frame: &mut Frame, area: ratatui::layout::Rect) {
        let help_text = "Enter: next step | Backspace: previous step | Ctrl+S: save | Ctrl+C: cancel";

        let paragraph = Paragraph::new(help_text)
            .style(Style::default().fg(Color::Rgb(107, 114, 128)))
            .alignment(ratatui::layout::Alignment::Center);

        frame.render_widget(paragraph, area);
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

    fn test_next_provider() {
        let mut wizard = ProviderWizardScreen::new();
        wizard.update_input_from_config();
        wizard.next_provider();
        assert_eq!(wizard.config.provider, ProviderKind::Anthropic);
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
        assert_eq!(wizard.validate_field(), ValidationResult::Invalid("Provider ID cannot be empty".to_string()));
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