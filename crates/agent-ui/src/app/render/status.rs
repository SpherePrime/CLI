use ratatui::prelude::*;
use ratatui::text::{Line, Span};
use ratatui::widgets::Paragraph;

use crate::theme::Theme;

use super::super::AgentApp;

pub fn render_status_bar(f: &mut Frame, area: Rect, app: &AgentApp, theme: &Theme) {
    if area.width < 10 {
        return;
    }

    let cwd = std::env::current_dir()
        .map(|p| p.to_string_lossy().to_string())
        .unwrap_or_default();
    let cwd_short = crate::ui::components::shorten_path(&cwd, 36);

    let left = Line::from(vec![
        Span::styled(
            "●",
            Style::default().fg(theme.status_ready),
        ),
        Span::styled(" ", Style::default()),
        Span::styled(
            app.title.clone(),
            Style::default().fg(theme.foreground).add_modifier(Modifier::BOLD),
        ),
        Span::styled(" ", Style::default()),
        Span::styled(
            cwd_short,
            Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM),
        ),
    ]);

    let right = Line::from(vec![
        Span::styled(
            "model",
            Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM),
        ),
        Span::styled(" ", Style::default()),
        Span::styled(
            app.model_config.model.clone(),
            Style::default().fg(theme.accent),
        ),
        Span::styled("  ", Style::default()),
        Span::styled(
            "provider",
            Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM),
        ),
        Span::styled(" ", Style::default()),
        Span::styled(
            super::super::provider_name(&app.model_config.provider),
            Style::default().fg(theme.accent_light),
        ),
    ]);

    let left_len = left.width() as u16;
    let max_right = area.width.saturating_sub(left_len).saturating_sub(2);

    if max_right >= right.width() as u16 {
        f.render_widget(Paragraph::new(right), Rect {
            x: area.x + left_len + 2,
            y: area.y,
            width: max_right,
            height: 1,
        });
    }

    f.render_widget(Paragraph::new(left), area);
}