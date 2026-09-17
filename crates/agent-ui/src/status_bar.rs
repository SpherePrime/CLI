use ratatui::style::Style;
use ratatui::text::{Line, Span};
use ratatui::widgets::Paragraph;

pub struct StatusBar {
    pub model: String,
    pub project: String,
    pub mode: String,
    pub context_used: usize,
}

impl StatusBar {
    pub fn new() -> Self {
        Self {
            model: "unknown".into(),
            project: "-".into(),
            mode: "interactive".into(),
            context_used: 0,
        }
    }

    pub fn render(&self) -> Paragraph {
        Paragraph::new(Line::from(vec![
            Span::styled(" model ", Style::default().fg(ratatui::style::Color::Cyan)),
            Span::raw(&self.model),
            Span::styled(" | project ", Style::default().fg(ratatui::style::Color::Yellow)),
            Span::raw(&self.project),
            Span::styled(" | mode ", Style::default().fg(ratatui::style::Color::Green)),
            Span::raw(&self.mode),
            Span::styled(
                format!(" | ctx {}/100%", self.context_used),
                Style::default().fg(ratatui::style::Color::White),
            ),
        ]))
    }
}

impl Default for StatusBar {
    fn default() -> Self {
        Self::new()
    }
}
