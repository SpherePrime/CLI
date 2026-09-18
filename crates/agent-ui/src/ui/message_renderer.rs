use ratatui::layout::Rect;
use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{ListItem, Paragraph};
use ratatui::Frame;
use std::borrow::Cow;

use crate::app::StatusType;
use crate::theme::Theme;

#[derive(Debug, Clone, PartialEq)]
pub enum MsgRole {
    User,
    Assistant,
    Tool,
    System,
}

impl MsgRole {
    pub fn as_str(&self) -> &'static str {
        match self {
            MsgRole::User => "user",
            MsgRole::Assistant => "assistant",
            MsgRole::Tool => "tool",
            MsgRole::System => "system",
        }
    }

    pub fn icon(&self, color: Color) -> Span<'static> {
        Span::styled("❯", Style::default().fg(color))
    }

    pub fn label(&self) -> &'static str {
        match self {
            MsgRole::User => "USER",
            MsgRole::Assistant => "AGENT",
            MsgRole::Tool => "TOOL",
            MsgRole::System => "RESULT",
        }
    }

    pub fn label_style(&self, color: Color) -> Style {
        Style::default().fg(color).add_modifier(Modifier::BOLD)
    }
}

#[derive(Debug, Clone)]
pub struct MsgEntry {
    pub role: MsgRole,
    pub content: String,
    pub tool_call: Option<String>,
    pub finished: bool,
}

impl MsgEntry {
    pub fn new(role: MsgRole, content: impl Into<String>) -> Self {
        Self {
            role,
            content: content.into(),
            tool_call: None,
            finished: true,
        }
    }

    pub fn with_tool_call(mut self, name: impl Into<String>) -> Self {
        self.tool_call = Some(name.into());
        self
    }
}

pub fn render_message(f: &mut Frame, area: Rect, msg: &MsgEntry, theme: &Theme) -> Rect {
    let lines = build_message_lines(
        msg,
        theme.message_user,
        theme.message_assistant,
        theme.message_tool,
        theme.success,
        theme.foreground,
        theme.info,
        theme.accent_light,
    );
    let para = Paragraph::new(lines)
        .style(Style::default().fg(theme.foreground))
        .wrap(ratatui::widgets::Wrap { trim: true });

    f.render_widget(para, area);
    area
}

#[allow(clippy::too_many_arguments)]
pub fn build_message_lines(
    msg: &MsgEntry,
    msg_user: Color,
    msg_assistant: Color,
    msg_tool: Color,
    msg_system: Color,
    fg: Color,
    info: Color,
    accent_light: Color,
) -> Vec<Line<'static>> {
    let mut lines = Vec::new();

    let (icon, label_style) = match msg.role {
        MsgRole::User => (msg.role.icon(msg_user), msg.role.label_style(msg_user)),
        MsgRole::Assistant => (
            msg.role.icon(msg_assistant),
            msg.role.label_style(msg_assistant),
        ),
        MsgRole::Tool => (msg.role.icon(msg_tool), msg.role.label_style(msg_tool)),
        MsgRole::System => (msg.role.icon(msg_system), msg.role.label_style(msg_system)),
    };

    let label = msg.role.label();
    let label_str: Cow<'static, str> = Cow::Owned(format!("{}:", label));
    let content_str: Cow<'static, str> = Cow::Owned(msg.content.clone());

    lines.push(Line::from(""));
    lines.push(Line::from(vec![
        Span::styled(" ", Style::default()),
        icon,
        Span::styled(" ", Style::default()),
        Span::styled(label_str, label_style),
        Span::styled(" ", Style::default()),
        Span::styled(content_str, Style::default().fg(fg)),
    ]));

    if let Some(tool_name) = &msg.tool_call {
        let tool_name_cow: Cow<'static, str> = Cow::Owned(tool_name.clone());
        lines.push(Line::from(vec![
            Span::styled("  → ", Style::default().fg(info)),
            Span::styled(tool_name_cow, Style::default().fg(accent_light)),
        ]));
    }

    lines.push(Line::from(""));
    lines
}

pub fn render_messages(f: &mut Frame, area: Rect, messages: &[MsgEntry], theme: &Theme) -> Rect {
    let mut all_lines: Vec<Line<'static>> = vec![Line::from("")];

    for msg in messages {
        all_lines.extend(build_message_lines(
            msg,
            theme.message_user,
            theme.message_assistant,
            theme.message_tool,
            theme.success,
            theme.foreground,
            theme.info,
            theme.accent_light,
        ));
    }

    let para = Paragraph::new(all_lines)
        .style(Style::default().fg(theme.foreground))
        .wrap(ratatui::widgets::Wrap { trim: true });

    f.render_widget(para, area);
    area
}

pub fn build_message_list_item(
    msg: &MsgEntry,
    fg: Color,
    msg_user: Color,
    msg_assistant: Color,
    msg_tool: Color,
    msg_system: Color,
) -> ListItem<'static> {
    let icon = match msg.role {
        MsgRole::User => msg.role.icon(msg_user),
        MsgRole::Assistant => msg.role.icon(msg_assistant),
        MsgRole::Tool => msg.role.icon(msg_tool),
        MsgRole::System => msg.role.icon(msg_system),
    };
    let label = msg.role.label();

    let preview = if msg.content.len() > 60 {
        format!("{}...", &msg.content[..57])
    } else {
        msg.content.clone()
    };

    let content = format!("<{}> {}", label, preview);

    let content_cow: Cow<'static, str> = Cow::Owned(content);

    ListItem::new(Line::from(vec![
        icon,
        Span::styled(" ", Style::default()),
        Span::styled(content_cow, Style::default().fg(fg)),
    ]))
}

pub fn build_status_span(status_type: &StatusType, theme: &Theme) -> Line<'static> {
    match status_type {
        StatusType::Ready => {
            let color: Color = theme.status_ready;
            Line::from(vec![Span::styled("● Ready", Style::default().fg(color))])
        }
        StatusType::Thinking(t) => {
            let text = if t.is_empty() {
                "◌ Thinking...".to_string()
            } else {
                format!("◌ {}", t)
            };
            let text_cow: Cow<'static, str> = Cow::Owned(text);
            let color: Color = theme.status_thinking;
            Line::from(vec![Span::styled(text_cow, Style::default().fg(color))])
        }
        StatusType::Success => {
            let color: Color = theme.status_success;
            Line::from(vec![Span::styled("✓ Done", Style::default().fg(color))])
        }
        StatusType::Error(e) => {
            let text = if e.is_empty() {
                "✕ Error".to_string()
            } else {
                format!("✕ Error: {}", e)
            };
            let text_cow: Cow<'static, str> = Cow::Owned(text);
            let color: Color = theme.status_error;
            Line::from(vec![Span::styled(text_cow, Style::default().fg(color))])
        }
        StatusType::Tool(name) => {
            let text = format!("→ {}", name);
            let text_cow: Cow<'static, str> = Cow::Owned(text);
            let color: Color = theme.message_tool;
            Line::from(vec![Span::styled(text_cow, Style::default().fg(color))])
        }
    }
}

pub fn render_status(_frame: &mut Frame, _area: Rect, _status_type: &StatusType, _theme: &Theme) {}
