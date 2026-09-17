 use ratatui::layout::Rect;
 use ratatui::prelude::*;
 use ratatui::text::{Line, Span};
 use ratatui::widgets::{Paragraph, Wrap};
 use ratatui::Frame;
 
 use crate::theme::Theme;
 
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
 
     let truncated = format!("~{}/{}/{first}", second_last, last);
     truncated
 }
 
 pub fn format_path(path: &str, _width: usize) -> String {
     if path.len() <= 40 {
         return path.to_string();
     }
 
     let parts: Vec<&str> = path.split('\\').collect();
     let last = parts.last().unwrap_or(&"");
     let first = parts.first().unwrap_or(&"");
 
     format!("~{first}/{last}")
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
     let status_line =
         format!("✓ Ready  model  {model}  provider  {provider}  📁 {cwd_short}");
 
     let header = Paragraph::new(status_line)
         .style(Style::default().fg(theme.foreground))
         .wrap(Wrap { trim: true });
 
     frame.render_widget(header, area);
 }
 
 pub fn render_status_line(
     _frame: &mut Frame,
     _area: Rect,
     _status_text: &str,
     _theme: &Theme,
 ) {
 }
 
 pub fn render_prompt(frame: &mut Frame, area: Rect, input: &str, theme: &Theme) {
     let prompt = Paragraph::new(Line::from(vec![
         Span::styled(
             "❯ ",
             Style::default().fg(theme.accent).add_modifier(Modifier::BOLD),
         ),
         Span::styled(input, Style::default().fg(theme.foreground)),
     ]))
     .style(Style::default().fg(theme.foreground));
 
     frame.render_widget(prompt, area);
 }
 
 pub fn render_empty_state(frame: &mut Frame, area: Rect, theme: &Theme) {
     let lines: Vec<Line> = vec![
         Line::from(""),
         Line::from(vec![Span::styled(
             "Ready to code.",
             Style::default().fg(theme.foreground).add_modifier(Modifier::BOLD),
         )]),
         Line::from(""),
         Line::from(vec![Span::styled(
             "Ask me to inspect, edit, refactor, test,",
             Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM),
         )]),
         Line::from(vec![Span::styled(
             "debug, or explain your code.",
             Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM),
         )]),
         Line::from(""),
         Line::from(vec![Span::styled(
             "Try: \"inspect this project for issues\"",
             Style::default().fg(theme.accent),
         )]),
         Line::from(""),
     ];
 
     let empty_state = Paragraph::new(lines)
         .style(Style::default().fg(theme.foreground))
         .wrap(Wrap { trim: true });
 
     frame.render_widget(empty_state, area);
 }
 
 pub fn render_help(
     _frame: &mut Frame,
     _area: Rect,
     _theme: &Theme,
     _commands: &[(String, String)],
 ) {
 }
