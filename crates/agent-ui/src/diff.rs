use ratatui::prelude::*;
use ratatui::widgets::{Paragraph, Wrap};

pub fn render_diff(f: &mut Frame, original: &str, modified: &str, area: Rect) {
    let ops = similar::TextDiff::from_lines(original, modified);
    let mut lines = Vec::new();
    for change in ops.iter_all_changes() {
        let marker = match change.tag() {
            similar::ChangeTag::Delete => "-",
            similar::ChangeTag::Insert => "+",
            similar::ChangeTag::Equal => " ",
        };
        let style = match change.tag() {
            similar::ChangeTag::Delete => Style::default().fg(Color::Red),
            similar::ChangeTag::Insert => Style::default().fg(Color::Green),
            similar::ChangeTag::Equal => Style::default().fg(Color::Gray),
        };
        lines.push(
            Paragraph::new(Line::from(Span::styled(
                format!("{marker}{}", change),
                style,
            )))
            .wrap(Wrap { trim: false }),
        );
    }
    for line in lines {
        f.render_widget(line, area);
    }
}

pub fn render_message(f: &mut Frame, message: &str, area: Rect, color: Color) {
    let p = Paragraph::new(Line::from(Span::styled(
        message,
        Style::default().fg(color),
    )))
    .wrap(Wrap { trim: true });
    f.render_widget(p, area);
}

pub struct DiffView {
    pub original: String,
    pub modified: String,
}

impl DiffView {
    pub fn new(original: impl Into<String>, modified: impl Into<String>) -> Self {
        Self {
            original: original.into(),
            modified: modified.into(),
        }
    }

    pub fn render_to_string(&self) -> String {
        let ops = similar::TextDiff::from_lines(&self.original, &self.modified);
        ops.unified_diff().to_string()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn diff_view() {
        let v = DiffView::new("a\nb\nc", "a\nB\nc");
        let s = v.render_to_string();
        assert!(s.contains("+B"));
    }
}
