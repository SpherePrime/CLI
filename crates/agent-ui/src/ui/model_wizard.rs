use agent_config::{ModelConfig, ProviderKind};
use ratatui::layout::{Constraint, Direction, Layout};
use ratatui::style::{Color, Modifier, Style};
use ratatui::widgets::{Block, BorderType, List, ListItem, Paragraph};
use ratatui::Frame;

use super::Component;

#[derive(Debug, Clone, PartialEq)]
pub enum WizardField {
    Provider,
    Model,
    BaseUrl,
    ApiKeyEnv,
    Temperature,
    MaxTokens,
}

pub struct ModelWizardScreen {
    pub config: ModelConfig,
    pub fields: Vec<String>,
    pub current_field: WizardField,
    pub provider_options: Vec<String>,
    pub is_editable: bool,
}

impl Default for ModelWizardScreen {
    fn default() -> Self {
        Self::new()
    }
}

impl ModelWizardScreen {
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
            fields: vec![],
            current_field: WizardField::Model,
            provider_options: vec!["openai".to_string(), "anthropic".to_string(), "google".to_string(), "mock".to_string(), "custom / openai-compatible".to_string()],
            is_editable: false,
        }
    }

    pub fn from_config(config: ModelConfig) -> Self {
        let mut wizard = Self::new();
        wizard.config = config;
        wizard
    }

    pub fn field_index(&self) -> usize {
        match self.current_field {
            WizardField::Provider => 0,
            WizardField::Model => 1,
            WizardField::BaseUrl => 2,
            WizardField::ApiKeyEnv => 3,
            WizardField::Temperature => 4,
            WizardField::MaxTokens => 5,
        }
    }

    pub fn select_next_field(&mut self) {
        self.current_field = match self.current_field {
            WizardField::Provider => WizardField::Model,
            WizardField::Model => WizardField::BaseUrl,
            WizardField::BaseUrl => WizardField::ApiKeyEnv,
            WizardField::ApiKeyEnv => WizardField::Temperature,
            WizardField::Temperature => WizardField::MaxTokens,
            WizardField::MaxTokens => WizardField::Provider,
        };
    }

    pub fn select_previous_field(&mut self) {
        self.current_field = match self.current_field {
            WizardField::Provider => WizardField::MaxTokens,
            WizardField::Model => WizardField::Provider,
            WizardField::BaseUrl => WizardField::Model,
            WizardField::ApiKeyEnv => WizardField::BaseUrl,
            WizardField::Temperature => WizardField::ApiKeyEnv,
            WizardField::MaxTokens => WizardField::Temperature,
        };
    }

    pub fn get_provider_index(&self) -> usize {
        self.provider_options
            .iter()
            .position(|p| p == provider_name(&self.config.provider))
            .unwrap_or(0)
    }

    pub fn set_provider_index(&mut self, index: usize) {
        if index < self.provider_options.len() {
            let provider = &self.provider_options[index];
            self.config.provider = match provider.as_str() {
                "openai" => ProviderKind::OpenAi,
                "anthropic" => ProviderKind::Anthropic,
                "google" => ProviderKind::Google,
                "mock" => ProviderKind::Mock,
                "custom / openai-compatible" => ProviderKind::OpenAiCompatible,
                _ => ProviderKind::OpenAi,
            };
        }
    }

    pub fn next_provider(&mut self) {
        let new_index = (self.get_provider_index() + 1) % self.provider_options.len();
        self.set_provider_index(new_index);
    }

    pub fn previous_provider(&mut self) {
        let current = self.get_provider_index();
        let new_index = if current == 0 {
            self.provider_options.len() - 1
        } else {
            current - 1
        };
        self.set_provider_index(new_index);
    }

    pub fn get_field_value(&self, field: &WizardField) -> String {
        match field {
            WizardField::Provider => provider_name(&self.config.provider).to_string(),
            WizardField::Model => self.config.model.clone(),
            WizardField::BaseUrl => self.config.base_url.clone().unwrap_or_default(),
            WizardField::ApiKeyEnv => self.config.api_key_env.clone().unwrap_or_default(),
            WizardField::Temperature => self.config.temperature.map(|t| t.to_string()).unwrap_or_default(),
            WizardField::MaxTokens => self.config.max_tokens.map(|t| t.to_string()).unwrap_or_default(),
        }
    }

    pub fn set_field_value(&mut self, field: &WizardField, value: String) {
        match field {
            WizardField::Provider => {}
            WizardField::Model => self.config.model = value,
            WizardField::BaseUrl => {
                if value.is_empty() {
                    self.config.base_url = None;
                } else {
                    self.config.base_url = Some(value);
                }
            }
            WizardField::ApiKeyEnv => {
                if value.is_empty() {
                    self.config.api_key_env = None;
                } else {
                    self.config.api_key_env = Some(value);
                }
            }
            WizardField::Temperature => {
                if value.is_empty() {
                    self.config.temperature = None;
                } else {
                    self.config.temperature = value.parse().ok();
                }
            }
            WizardField::MaxTokens => {
                if value.is_empty() {
                    self.config.max_tokens = None;
                } else {
                    self.config.max_tokens = value.parse().ok();
                }
            }
        }
    }

    pub fn can_save(&self) -> bool {
        !self.config.model.is_empty()
    }

    pub fn build_config(&self) -> ModelConfig {
        self.config.clone()
    }
}

impl Component for ModelWizardScreen {
    fn render(&self, frame: &mut Frame, area: ratatui::layout::Rect) -> ratatui::layout::Rect {
        let main_block = Block::bordered()
            .title("Model Wizard - Configure your AI model")
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

impl ModelWizardScreen {
    fn render_form(&self, frame: &mut Frame, area: ratatui::layout::Rect) {
        let fields = [
            (WizardField::Provider, "Provider"),
            (WizardField::Model, "Model ID"),
            (WizardField::BaseUrl, "Base URL"),
            (WizardField::ApiKeyEnv, "API Key Env"),
            (WizardField::Temperature, "Temperature"),
            (WizardField::MaxTokens, "Max Tokens"),
        ];

        let chunk_height = area.height.saturating_sub(fields.len() as u16) as usize;
        let chunk_height = chunk_height.max(1);

        for (i, (field, label)) in fields.iter().enumerate() {
            let field_y_offset = (i * (chunk_height + 1)) as u16;
            let field_area = ratatui::layout::Rect {
                x: area.x,
                y: area.y.saturating_add(field_y_offset),
                width: area.width,
                height: chunk_height as u16,
            };

            let is_active = *field == self.current_field;
            let style = if is_active {
                Style::default()
                    .fg(Color::Rgb(118, 158, 242))
                    .add_modifier(Modifier::BOLD)
            } else {
                Style::default().fg(Color::Rgb(107, 114, 128))
            };

            let value = self.get_field_value(field);
            let placeholder = self.get_placeholder(field);

            let display_line = if is_active {
                format!("▶ {}: {} ({})", label, value, placeholder)
            } else {
                format!("  {}: {}", label, if value.is_empty() { "-" } else { &value })
            };

            let paragraph = Paragraph::new(display_line).style(style);

            let block = Block::bordered()
                .border_type(BorderType::Rounded)
                .style(style);

            let inner = block.inner(field_area);
            
            frame.render_widget(block, field_area);
            frame.render_widget(paragraph, inner);
        }
    }

    fn get_placeholder(&self, field: &WizardField) -> &'static str {
        match field {
            WizardField::Provider => "Tab/Shift+Tab to change",
            WizardField::Model => "model name (e.g. gpt-4)",
            WizardField::BaseUrl => "http://... (optional)",
            WizardField::ApiKeyEnv => "e.g. OPENAI_API_KEY",
            WizardField::Temperature => "0.0-2.0",
            WizardField::MaxTokens => "e.g. 4096",
        }
    }

    fn render_preview(&self, frame: &mut Frame, area: ratatui::layout::Rect) {
        let block = Block::bordered()
            .title("Preview")
            .border_type(BorderType::Rounded)
            .style(Style::default().fg(Color::Rgb(156, 163, 175)).add_modifier(Modifier::ITALIC));

        let inner = block.inner(area);

        let model_name = provider_name(&self.config.provider);
        let model_id = if self.config.model.is_empty() { "-" } else { &self.config.model };

        let mut lines = vec![
            format!("Provider: {}", model_name),
            format!("Model: {}", model_id),
        ];

        if let Some(ref url) = self.config.base_url {
            lines.push(format!("Base URL: {}", url));
        }

        if let Some(ref key) = self.config.api_key_env {
            lines.push(format!("API Key Env: {}", key));
        }

        if let Some(temp) = self.config.temperature {
            lines.push(format!("Temperature: {}", temp));
        }

        if let Some(tokens) = self.config.max_tokens {
            lines.push(format!("Max Tokens: {}", tokens));
        }

        let list_items: Vec<ListItem> = lines
            .iter()
            .map(|l| ListItem::new(l.as_str()).style(Style::default().fg(Color::Rgb(203, 213, 224))))
            .collect();

        let list = List::new(list_items).style(Style::default().fg(Color::Rgb(203, 213, 224)));

        frame.render_widget(block, area);
        frame.render_widget(list, inner);
    }

    fn render_status(&self, frame: &mut Frame, area: ratatui::layout::Rect) {
        let help_text = "Tab: next field | Shift+Tab: prev field | Enter: set value | Ctrl+S: save | Ctrl+C: cancel";

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

pub fn run_model_wizard(_current: Option<&ModelConfig>) -> ModelConfig {
    ModelWizardScreen::new().build_config()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_model_wizard_screen_new() {
        let wizard = ModelWizardScreen::new();
        assert_eq!(wizard.current_field, WizardField::Model);
        assert_eq!(wizard.config.provider, ProviderKind::Mock);
    }

    #[test]
    fn test_field_index() {
        let wizard = ModelWizardScreen::new();
        assert_eq!(wizard.field_index(), 1);
    }

    #[test]
    fn test_select_next_field() {
        let mut wizard = ModelWizardScreen::new();
        assert_eq!(wizard.current_field, WizardField::Model);
        wizard.select_next_field();
        assert_eq!(wizard.current_field, WizardField::BaseUrl);
    }

    #[test]
    fn test_select_previous_field() {
        let mut wizard = ModelWizardScreen::new();
        wizard.select_previous_field();
        assert_eq!(wizard.current_field, WizardField::Provider);
    }

    #[test]
    fn test_provider_index() {
        let wizard = ModelWizardScreen::new();
        assert_eq!(wizard.get_provider_index(), 3);
    }

    #[test]
    fn test_next_provider() {
        let mut wizard = ModelWizardScreen::new();
        wizard.next_provider();
        assert_eq!(wizard.config.provider, ProviderKind::OpenAiCompatible);
    }

    #[test]
    fn test_previous_provider() {
        let mut wizard = ModelWizardScreen::new();
        wizard.previous_provider();
        assert_eq!(wizard.config.provider, ProviderKind::Google);
    }

    #[test]
    fn test_get_set_field_value() {
        let mut wizard = ModelWizardScreen::new();
        wizard.set_field_value(&WizardField::Model, "gpt-4".to_string());
        assert_eq!(wizard.get_field_value(&WizardField::Model), "gpt-4");
    }

    #[test]
    fn test_can_save() {
        let wizard = ModelWizardScreen::new();
        assert!(wizard.can_save());
    }

    #[test]
    fn test_can_save_with_model() {
        let mut wizard = ModelWizardScreen::new();
        wizard.config.model = "gpt-4".to_string();
        assert!(wizard.can_save());
    }

    #[test]
    fn test_build_config() {
        let mut wizard = ModelWizardScreen::new();
        wizard.config.model = "gpt-4".to_string();
        wizard.config.provider = ProviderKind::OpenAi;
        let config = wizard.build_config();
        assert_eq!(config.model, "gpt-4");
        assert_eq!(config.provider, ProviderKind::OpenAi);
    }
}