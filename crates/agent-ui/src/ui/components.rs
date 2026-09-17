use ratatui::layout::{Constraint, Direction, Layout, Rect};
use ratatui::prelude::*;
use ratatui::widgets::{Block, BorderType, Paragraph, Wrap, List};
use ratatui::text::{Line, Span};
use ratatui::Frame;

use crate::theme::Theme;

pub trait Component {
    fn render(&self, frame: &mut Frame, area: Rect) -> Rect;
}

#[derive(Debug, Clone)]
pub struct InputField {
    pub label: String,
    pub placeholder: String,
    pub value: String,
}

impl InputField {
    pub fn new(label: &str) -> Self {
        Self {
            label: label.to_string(),
            placeholder: String::new(),
            value: String::new(),
        }
    }

    pub fn placeholder(mut self, placeholder: &str) -> Self {
        self.placeholder = placeholder.to_string();
        self
    }

    pub fn default(mut self, value: &str) -> Self {
        self.value = value.to_string();
        self
    }

    pub fn set_value(&mut self, value: &str) {
        self.value = value.to_string();
    }
}

impl Component for InputField {
    fn render(&self, frame: &mut Frame, area: Rect) -> Rect {
        let display_text = if self.value.is_empty() {
            self.placeholder.clone()
        } else {
            self.value.clone()
        };

        let block = Block::bordered()
            .title(self.label.as_str())
            .border_type(BorderType::Rounded);

        let inner_area = block.inner(area);
        frame.render_widget(block, area);

        let paragraph = Paragraph::new(display_text);
        frame.render_widget(paragraph, inner_area);

        inner_area
    }
}

#[derive(Debug, Clone)]
pub struct SelectList {
    pub label: String,
    pub options: Vec<String>,
    pub selected_index: usize,
}

impl SelectList {
    pub fn new(label: &str) -> Self {
        Self {
            label: label.to_string(),
            options: Vec::new(),
            selected_index: 0,
        }
    }

    pub fn options(mut self, options: Vec<&str>) -> Self {
        self.options = options.into_iter().map(|s| s.to_string()).collect();
        self
    }

    pub fn default(mut self, value: &str) -> Self {
        if let Some(idx) = self.options.iter().position(|o| o == value) {
            self.selected_index = idx;
        }
        self
    }

    pub fn selected(&self) -> Option<&str> {
        self.options.get(self.selected_index).map(|s| s.as_str())
    }

    pub fn set_selected(&mut self, index: usize) {
        if index < self.options.len() {
            self.selected_index = index;
        }
    }

    pub fn previous(&mut self) {
        if self.selected_index > 0 {
            self.selected_index -= 1;
        }
    }

    pub fn next(&mut self) {
        if self.selected_index < self.options.len().saturating_sub(1) {
            self.selected_index += 1;
        }
    }
}

impl Component for SelectList {
    fn render(&self, frame: &mut Frame, area: Rect) -> Rect {
        let items: Vec<_> = self
            .options
            .iter()
            .enumerate()
            .map(|(i, opt)| {
                if i == self.selected_index {
                    format!("▶ {}", opt)
                } else {
                    format!("  {}", opt)
                }
            })
            .collect();

        let block = Block::bordered()
            .title(self.label.as_str())
            .border_type(BorderType::Rounded);

        let inner_area = block.inner(area);

        frame.render_widget(block, area);

        let list = List::new(items);
        frame.render_widget(list, inner_area);

        inner_area
    }
}

#[derive(Debug, Clone)]
pub struct FormField {
    pub input: InputField,
    pub select: SelectList,
}

impl FormField {
    pub fn new(input_label: &str, select_label: &str) -> Self {
        Self {
            input: InputField::new(input_label),
            select: SelectList::new(select_label),
        }
    }

    pub fn input(mut self, placeholder: &str, default: &str) -> Self {
        self.input = self.input.placeholder(placeholder).default(default);
        self
    }

    pub fn select(mut self, options: Vec<&str>, default: &str) -> Self {
        self.select = self.select.options(options).default(default);
        self
    }
}

impl Component for FormField {
    fn render(&self, frame: &mut Frame, area: Rect) -> Rect {
        let block = Block::bordered()
            .title("Form")
            .border_type(BorderType::Rounded);

        let area = block.inner(area);
        frame.render_widget(block, area);

        let input_height = 3u16;
        let select_area = Rect {
            x: area.x,
            y: area.y + input_height,
            width: area.width,
            height: area.height.saturating_sub(input_height),
        };

        self.select.render(frame, select_area);

        area
    }
}

pub fn shorten_path(path: &str, max_len: usize) -> String {
    let path_str = if path.starts_with("C:\\") || path.starts_with("D:\\") || path.starts_with("E:\\") {
        let rest = &path[2..];
        if rest.starts_with("\\") {
            &rest[1..]
        } else {
            path
        }
    } else if path.starts_with('/') {
        &path[1..]
    } else {
        path
    };

    if path_str.len() <= max_len {
        return path_str.to_string();
    }

    let parts: Vec<&str> = path_str.split('\\').collect();
    if parts.len() <= 2 {
        return format!("{}/...", &path_str[..max_len.saturating_sub(4)]);
    }

    let last = parts.last().unwrap_or(&"");
    let second_last = parts.get(parts.len().saturating_sub(2)).unwrap_or(&"");
    let first = parts.first().unwrap_or(&"");

    let truncated = format!("~{}/{}/{}", first, second_last, last);
    if truncated.len() <= max_len {
        return truncated;
    }

    truncated
}

pub fn format_path(path: &str, _width: usize) -> String {
    if path.len() <= 40 {
        return path.to_string();
    }
    
    let parts: Vec<&str> = path.split('\\').collect();
    let last = parts.last().unwrap_or(&"");
    let first = parts.first().unwrap_or(&"");
    
    format!("~{}/{}", first, last)
}

pub fn render_header(
    frame: &mut Frame,
    area: Rect,
    model: &str,
    provider: &str,
    cwd: &str,
    theme: &Theme,
) {
    let cwd_short = shorten_path(cwd, 30);
    let status_line = format!("✓ Ready  model  {}  provider  {}  📁 {}", 
        model, provider, cwd_short);
    
    let header = Paragraph::new(status_line)
        .style(Style::default().fg(theme.foreground))
        .wrap(Wrap { trim: true });

    frame.render_widget(header, area);
}

pub fn render_status_line(_frame: &mut Frame, area: Rect, status_text: &str, theme: &Theme) {
    let status = Paragraph::new(status_text)
        .style(Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM))
        .wrap(Wrap { trim: true });

    let _ = status;
    let _ = area;
}

pub fn render_prompt(frame: &mut Frame, area: Rect, input: &str, theme: &Theme) {
    let prompt = Paragraph::new(Line::from(vec![
        Span::styled("❯ ", Style::default().fg(theme.accent).add_modifier(Modifier::BOLD)),
        Span::styled(input, Style::default().fg(theme.foreground)),
    ]))
    .style(Style::default().fg(theme.foreground));

    frame.render_widget(prompt, area);
}

pub fn render_empty_state(frame: &mut Frame, area: Rect, theme: &Theme) {
    let lines: Vec<Line> = vec![
        Line::from(""),
        Line::from(vec![
            Span::styled("Ready to code.", Style::default().fg(theme.foreground).add_modifier(Modifier::BOLD)),
        ]),
        Line::from(""),
        Line::from(vec![
            Span::styled("Ask me to inspect, edit, refactor, test,", Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM)),
        ]),
        Line::from(vec![
            Span::styled("debug, or explain your code.", Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM)),
        ]),
        Line::from(""),
        Line::from(vec![
            Span::styled("Try: \"inspect this project for issues\"", Style::default().fg(theme.accent)),
        ]),
        Line::from(""),
    ];

    let empty_state = Paragraph::new(lines)
        .style(Style::default().fg(theme.foreground))
        .wrap(Wrap { trim: true });

    frame.render_widget(empty_state, area);
}

pub fn render_help(_frame: &mut Frame, _area: Rect, _theme: &Theme, _commands: &[(String, String)]) {
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_shorten_path() {
        let result = shorten_path("C:\\Users\\dwert\\OneDrive\\GitHub\\CLI", 30);
        assert!(result.contains('~'));
    }
}
