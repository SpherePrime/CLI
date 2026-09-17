use ratatui::layout::Rect;
use ratatui::style::{Color, Style};
use ratatui::widgets::{Block, BorderType, Paragraph};
use ratatui::Frame;

use super::Component;

#[derive(Debug, Clone)]
pub struct BannerScreen {
    pub model: String,
    pub provider: String,
    pub version: String,
    pub accent_color: Color,
}

impl BannerScreen {
    pub fn new() -> Self {
        Self {
            model: String::new(),
            provider: String::new(),
            version: "dev".to_string(),
            accent_color: Color::Rgb(118, 158, 242),
        }
    }

    pub fn with_model(mut self, model: &str) -> Self {
        self.model = model.to_string();
        self
    }

    pub fn with_provider(mut self, provider: &str) -> Self {
        self.provider = provider.to_string();
        self
    }

    pub fn with_version(mut self, version: &str) -> Self {
        self.version = version.to_string();
        self
    }

    pub fn with_accent(mut self, color: Color) -> Self {
        self.accent_color = color;
        self
    }
}

impl Default for BannerScreen {
    fn default() -> Self {
        Self::new()
    }
}

impl Component for BannerScreen {
    fn render(&self, _frame: &mut Frame, area: Rect) -> Rect {
        let title_line = " AGENT  —  AI Coding Agent";
        let model_provider_line = format!(" model: {}  ·  provider: {}", self.model, self.provider);
        let commands_line = " /help · /model · /config · /status · /tools · /exit";

        let lines = vec![
            title_line,
            &model_provider_line,
            &commands_line,
        ];

        let content = lines.join("\n");

        let block = Block::bordered()
            .title("═".repeat(48))
            .border_type(BorderType::Plain)
            .style(Style::default());

        let inner_area = block.inner(area);

        let paragraph = Paragraph::new(content)
            .style(Style::default())
            .block(block);

        // This is a simplified render - in real implementation would use frame.render_widget
        inner_area
    }
}

pub fn draw_welcome(model: &str, provider: &str, version: &str) -> BannerScreen {
    BannerScreen::new()
        .with_model(model)
        .with_provider(provider)
        .with_version(version)
}