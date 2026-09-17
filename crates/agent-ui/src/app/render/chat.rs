use ratatui::prelude::*;
use ratatui::text::{Line, Span};
use ratatui::widgets::Paragraph;

use super::super::{AgentApp, MessageRole};
use crate::theme::Theme;

pub fn render_chat_area(f: &mut Frame, area: Rect, app: &AgentApp, theme: &Theme) {
    if app.messages.is_empty() {
        render_empty_state(f, area, theme);
        return;
    }

    let lines = build_message_lines(app, theme, area.width);
    let total = lines.len();
    let visible = area.height as usize;

    let scroll_top = if app.stick_to_bottom {
        total.saturating_sub(visible)
    } else {
        let from_bottom = app.scroll_from_bottom;
        total.saturating_sub(visible).saturating_sub(from_bottom)
    };

    let para = Paragraph::new(lines)
        .style(Style::default().fg(theme.foreground))
        .wrap(ratatui::widgets::Wrap { trim: true })
        .scroll((scroll_top as u16, 0));

    f.render_widget(para, area);
}

fn build_message_lines(app: &AgentApp, theme: &Theme, width: u16) -> Vec<Line<'static>> {
    let mut out: Vec<Line<'static>> = Vec::new();
    out.push(Line::from(""));

    for msg in &app.messages {
        let (color, label, icon) = match msg.role {
            MessageRole::User => (theme.message_user, "you", "❯"),
            MessageRole::Assistant => (theme.message_assistant, "agent", "●"),
            MessageRole::Tool => (theme.message_tool, "tool", "→"),
            MessageRole::System => (theme.success, "system", "✓"),
        };

        out.push(Line::from(vec![
            Span::styled(" ", Style::default()),
            Span::styled(
                icon,
                Style::default().fg(color).add_modifier(Modifier::BOLD),
            ),
            Span::styled(" ", Style::default()),
            Span::styled(
                label,
                Style::default().fg(color).add_modifier(Modifier::BOLD),
            ),
        ]));

        let content_lines = wrap_content(&msg.content, width);
        for line in content_lines {
            out.push(Line::from(vec![
                Span::styled("  ", Style::default()),
                Span::styled(line, Style::default().fg(theme.foreground)),
            ]));
        }

        out.push(Line::from(""));
    }

    out
}

fn wrap_content(content: &str, width: u16) -> Vec<String> {
    let inner = width.saturating_sub(4).max(10) as usize;
    let mut out = Vec::new();
    for raw in content.lines() {
        if raw.is_empty() {
            out.push(String::new());
            continue;
        }
        let wrapped = textwrap::wrap(raw, inner);
        for piece in wrapped {
            out.push(piece.to_string());
        }
    }
    if !content.ends_with('\n') {
    }
    out
}

fn render_empty_state(f: &mut Frame, area: Rect, theme: &Theme) {
    let line = Line::from(vec![
        Span::styled(
            "Ready to code.",
            Style::default().fg(theme.foreground).add_modifier(Modifier::BOLD),
        ),
        Span::styled(" ", Style::default()),
        Span::styled(
            "Ask me to inspect, edit, refactor, test or debug code. Type",
            Style::default().fg(theme.accent_light),
        ),
        Span::styled(" /help ", Style::default().fg(theme.accent)),
        Span::styled("for commands.", Style::default().fg(theme.accent_light)),
    ]);

    let para = Paragraph::new(line).style(Style::default().fg(theme.foreground));
    f.render_widget(para, area);
}