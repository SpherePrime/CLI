use ratatui::layout::Rect;
use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{List, ListItem};
use ratatui::Frame;

use crate::theme::Theme;

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
            accent_color: Color::Rgb(79, 193, 255),
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

impl BannerScreen {
    pub fn render(&self, frame: &mut Frame, area: Rect) -> Rect {
        let theme = Theme::default_theme();
        
        let title_line = Line::from(vec![
            Span::styled("AI Coding Agent", Style::default().fg(self.accent_color).add_modifier(Modifier::BOLD)),
        ]);
        
        let model_line = Line::from(vec![
            Span::styled("model  ", Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM)),
            Span::styled(&self.model, Style::default().fg(self.accent_color)),
        ]);
        
        let provider_line = Line::from(vec![
            Span::styled("provider  ", Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM)),
            Span::styled(&self.provider, Style::default().fg(theme.accent_light)),
        ]);
        
        let version_line = Line::from(vec![
            Span::styled("version  ", Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM)),
            Span::styled(&self.version, Style::default().fg(theme.accent_light)),
        ]);

        let items: Vec<ListItem> = vec![
            ListItem::new(title_line),
            ListItem::new(model_line),
            ListItem::new(provider_line),
            ListItem::new(version_line),
        ];

        let list = List::new(items);
        
        frame.render_widget(list, area);
        
        area
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