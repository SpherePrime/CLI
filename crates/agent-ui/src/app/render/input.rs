use ratatui::prelude::*;
use ratatui::text::{Line, Span};
use ratatui::widgets::{Block, BorderType, Borders, List, ListItem, Paragraph};

use crate::completion::CompleteKind;
use crate::theme::Theme;
use ratatui::widgets::Wrap;

use super::super::AgentApp;

const PROMPT: &str = "~";

pub fn render_input_area(f: &mut Frame, area: Rect, app: &AgentApp, theme: &Theme) {
    let (input_area, menu_area) = split_with_menu(area, &app.completion);

    if menu_area.height > 0 {
        render_completion_menu(f, menu_area, app, theme);
    }

    let input_text = app.input();
    let lines = input_to_lines(input_text, input_area.width, theme);
    let cursor = cursor_position(app, input_area.width);

    let para = Paragraph::new(lines)
        .style(Style::default().fg(theme.foreground))
        .wrap(Wrap { trim: false });

    f.render_widget(para, input_area);

    let x = input_area.x.saturating_add(cursor.0);
    let y = input_area.y.saturating_add(cursor.1);
    f.set_cursor_position((x, y));
}

pub fn render_wizard_footer(
    f: &mut Frame,
    area: Rect,
    app: &AgentApp,
    status_text: &str,
    theme: &Theme,
) {
    let line = Line::from(vec![
        Span::styled(
            &app.editor.content,
            Style::default().fg(theme.foreground),
        ),
        Span::styled(" ", Style::default()),
        Span::styled(
            status_text,
            Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM),
        ),
    ]);
    f.render_widget(Paragraph::new(line), area);
}

fn split_with_menu(
    area: Rect,
    completion: &crate::completion::CompletionEngine,
) -> (Rect, Rect) {
    if !completion.visible() {
        return (area, Rect::default());
    }
    let menu_height = 8u16.min(completion.items.len() as u16);
    let input_height = area.height.saturating_sub(menu_height).max(1);
    let menu_area = Rect {
        x: area.x,
        y: area.y,
        width: area.width,
        height: menu_height,
    };
    let input_area = Rect {
        x: area.x,
        y: area.y + menu_height,
        width: area.width,
        height: input_height,
    };
    (input_area, menu_area)
}

fn input_to_lines(input: &str, width: u16, theme: &Theme) -> Vec<Line<'static>> {
    let inner = width.saturating_sub(PROMPT.chars().count() as u16 + 1).max(1) as usize;
    let mut lines = Vec::new();
    for raw in input.split('\n') {
        if raw.is_empty() {
            lines.push(Line::from(vec![
                Span::styled(PROMPT.to_string(), Style::default().fg(theme.accent)),
                Span::styled(" ", Style::default()),
            ]));
            continue;
        }
        let wrapped = textwrap::wrap(raw, inner);
        for (idx, piece) in wrapped.iter().enumerate() {
            let mut spans = Vec::new();
            if idx == 0 {
                spans.push(Span::styled(
                    PROMPT.to_string(),
                    Style::default().fg(theme.accent),
                ));
                spans.push(Span::styled(" ", Style::default()));
            }
            spans.push(Span::styled(
                piece.to_string(),
                Style::default().fg(theme.foreground),
            ));
            lines.push(Line::from(spans));
        }
    }
    if lines.is_empty() {
        lines.push(Line::from(vec![
            Span::styled(PROMPT.to_string(), Style::default().fg(theme.accent)),
            Span::styled(" ", Style::default()),
        ]));
    }
    lines
}

fn cursor_position(app: &AgentApp, width: u16) -> (u16, u16) {
    let inner = width.saturating_sub(PROMPT.chars().count() as u16 + 1).max(1) as usize;
    let before = &app.editor.content[..app.editor.cursor.min(app.editor.content.len())];

    let mut line = 0u16;
    let mut col = PROMPT.chars().count() as u16 + 1;

    for raw in before.split('\n') {
        if line == 0 {
            col = PROMPT.chars().count() as u16 + 1;
        }
        let wrapped = textwrap::wrap(raw, inner);
        if wrapped.is_empty() {
            if line > 0 {
                continue;
            }
        }
        let count = wrapped.len();
        if count > 0 {
            let last = raw.chars().count();
            let last_wrapped = wrapped[count - 1].chars().count();
            let is_full = last == last_wrapped && inner > 0 && last % inner == 0;
            let (used_lines, current) = if is_full {
                (count, last_wrapped)
            } else {
                (count.saturating_sub(1), last_wrapped)
            };
            line += used_lines as u16;
            col = PROMPT.chars().count() as u16 + 1 + current as u16;
        }
    }

    (col, line)
}

pub fn render_completion_menu(
    f: &mut Frame,
    area: Rect,
    app: &AgentApp,
    theme: &Theme,
) {
    if app.completion.items.is_empty() {
        return;
    }

    let kind_label = match app.completion.kind {
        CompleteKind::Slash => "Commands",
        CompleteKind::File => "Files",
        CompleteKind::None => "",
    };

    let block = Block::default()
        .borders(Borders::TOP)
        .border_type(BorderType::Rounded)
        .border_style(Style::default().fg(theme.border))
        .title(Span::styled(
            format!(" {kind_label} "),
            Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM),
        ));

    let list_area = block.inner(area);
    f.render_widget(block, area);

    let items: Vec<ListItem> = app
        .completion
        .items
        .iter()
        .enumerate()
        .map(|(i, item)| {
            let prefix = if i == app.completion.selected {
                "▶ "
            } else {
                "  "
            };
            let name_style = if i == app.completion.selected {
                Style::default()
                    .fg(theme.background)
                    .bg(theme.accent_light)
                    .add_modifier(Modifier::BOLD)
            } else {
                Style::default().fg(theme.foreground)
            };
            let content = Line::from(vec![
                Span::styled(prefix.to_string(), name_style),
                Span::styled(item.value.clone(), name_style),
                Span::styled(" ", Style::default()),
                Span::styled(
                    item.description.clone(),
                    if i == app.completion.selected {
                        Style::default()
                            .fg(theme.background)
                            .bg(theme.accent_light)
                            .add_modifier(Modifier::DIM)
                    } else {
                        Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM)
                    },
                ),
            ]);
            ListItem::new(content)
        })
        .collect();

    let list = List::new(items);
    f.render_widget(list, list_area);
}