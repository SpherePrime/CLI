use ratatui::layout::Rect;
use ratatui::style::{Color, Modifier, Style};
use ratatui::widgets::{Block, BorderType, List, ListItem};
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
        let line = "══════════════════════════════════════════════";
        let title_line = format!("  AGENT  —  AI Coding Agent  v{}", self.version);
        let model_provider_line = format!("  model: {}  ·  provider: {}", self.model, self.provider);
        let commands_line = "  /help · /model · /config · /status · /tools · /exit";

        let items: Vec<ListItem> = vec![
            ListItem::new(line).style(Style::default().fg(Color::Rgb(79, 193, 255)).add_modifier(Modifier::BOLD)),
            ListItem::new(title_line).style(Style::default().fg(Color::Rgb(79, 193, 255)).add_modifier(Modifier::BOLD)),
            ListItem::new(model_provider_line).style(Style::default().fg(Color::Rgb(107, 114, 128))),
            ListItem::new(commands_line).style(Style::default().fg(Color::Rgb(107, 114, 128))),
            ListItem::new(line).style(Style::default().fg(Color::Rgb(79, 193, 255)).add_modifier(Modifier::BOLD)),
        ];

        let banner_block = Block::bordered()
            .border_type(BorderType::Plain)
            .style(Style::default());

        let inner_area = banner_block.inner(area);
        let list = List::new(items).block(banner_block);

        // Simplified render - returns inner area
        inner_area
    }
}

pub fn draw_welcome(model: &str, provider: &str, version: &str) -> BannerScreen {
    BannerScreen::new()
        .with_model(model)
        .with_provider(provider)
        .with_version(version)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_banner_screen_new() {
        let banner = BannerScreen::new();
        assert_eq!(banner.model, "");
        assert_eq!(banner.provider, "");
        assert_eq!(banner.version, "dev");
    }

    #[test]
    fn test_banner_screen_with_data() {
        let banner = BannerScreen::new()
            .with_model("gpt-4")
            .with_provider("openai")
            .with_version("1.0.0");

        assert_eq!(banner.model, "gpt-4");
        assert_eq!(banner.provider, "openai");
        assert_eq!(banner.version, "1.0.0");
    }

    #[test]
    fn test_draw_welcome() {
        let banner = draw_welcome("gpt-4", "anthropic", "0.1.0");
        assert_eq!(banner.model, "gpt-4");
        assert_eq!(banner.provider, "anthropic");
        assert_eq!(banner.version, "0.1.0");
    }
}