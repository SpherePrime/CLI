 use ratatui::prelude::*;
 use ratatui::text::Span;
 use ratatui::widgets::{Paragraph, Wrap};
 
 use crate::app::state;
 use crate::theme::Theme;
 use crate::ui::banner;
 use crate::ui::components::render_header;
 
 use super::{AgentApp, MessageEntry, MessageRole};
 
 pub fn render_app(f: &mut Frame, _layout: Layout, app: &AgentApp) {
     let area = f.area();
     let theme = Theme::default_theme();
 
     match app.current_state {
         state::AppState::Banner => render_banner(f, area, app, &theme),
         state::AppState::Chat => render_chat(f, area, app, &theme),
         state::AppState::ModelWizard
         | state::AppState::ProviderWizard
         | state::AppState::ConfigMenu => render_wizard_status(f, area, app, &theme),
     }
 }
 
 fn render_banner(f: &mut Frame, area: Rect, app: &AgentApp, _theme: &Theme) {
     let banner = banner::BannerScreen::new()
         .with_model(&app.model_config.model)
         .with_provider(state::provider_name(&app.model_config.provider))
         .with_version(&app.version);
     banner.render(f, area);
 }
 
 fn render_chat(f: &mut Frame, area: Rect, app: &AgentApp, theme: &Theme) {
     let chunks = Layout::default()
         .direction(Direction::Vertical)
         .constraints([
             Constraint::Length(1),
             Constraint::Min(0),
             Constraint::Length(1),
         ])
         .split(area);
 
     let cwd = std::env::current_dir()
         .unwrap_or_default()
         .to_string_lossy()
         .to_string();
 
     render_header(
         f,
         chunks[0],
         &app.model_config.model,
         state::provider_name(&app.model_config.provider),
         &cwd,
         theme,
     );
 
     if app.messages.is_empty() {
         render_messages_list(f, chunks[1], &app.messages, theme);
     } else {
         render_message_history(f, chunks[1], &app.messages, theme);
     }
 
     let status_text = app.build_status_text();
     let footer = footer_line(theme, &app.input, &status_text);
     f.render_widget(footer, chunks[2]);
 }
 
 fn render_message_history(
     f: &mut Frame,
     area: Rect,
     messages: &[MessageEntry],
     theme: &Theme,
 ) {
     let mut lines: Vec<Line> = vec![Line::from("")];
 
     for msg in messages {
         let role_str = match msg.role {
             MessageRole::User => "USER",
             MessageRole::Assistant => "AGENT",
             MessageRole::Tool => "TOOL",
             MessageRole::System => "RESULT",
         };
 
         let color = match msg.role {
             MessageRole::User => theme.message_user,
             MessageRole::Assistant => theme.message_assistant,
             MessageRole::Tool => theme.message_tool,
             MessageRole::System => theme.success,
         };
 
         let icon = match msg.role {
             MessageRole::User => "❯",
             MessageRole::Assistant => "●",
             MessageRole::Tool => "→",
             MessageRole::System => "✓",
         };
 
         let preview = if msg.content.len() > 100 {
             format!("{}...", &msg.content[..97])
         } else {
             msg.content.clone()
         };
 
         lines.push(Line::from(vec![
             Span::styled(" ", Style::default()),
             Span::styled(icon, Style::default().fg(color)),
             Span::styled(" ", Style::default()),
             Span::styled(
                 role_str,
                 Style::default().fg(color).add_modifier(Modifier::BOLD),
             ),
             Span::styled(" ", Style::default()),
             Span::styled(preview, Style::default().fg(theme.foreground)),
         ]));
     }
 
     let messages_para = Paragraph::new(lines)
         .style(Style::default().fg(theme.foreground))
         .wrap(Wrap { trim: true });
 
     f.render_widget(messages_para, area);
 }
 
 fn render_messages_list(f: &mut Frame, area: Rect, _messages: &[MessageEntry], theme: &Theme) {
     let empty_lines: Vec<Line> = vec![
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
 
     let empty_para = Paragraph::new(empty_lines)
         .style(Style::default().fg(theme.foreground))
         .wrap(Wrap { trim: true });
 
     f.render_widget(empty_para, area);
 }
 
 fn render_wizard_status(f: &mut Frame, area: Rect, app: &AgentApp, theme: &Theme) {
     let status_text = app.build_status_text();
     let status = footer_line(theme, &app.input, &status_text);
     f.render_widget(status, area);
 }
 
fn footer_line<'a>(theme: &Theme, input: &'a str, status_text: &'a str) -> Paragraph<'a> {
     Paragraph::new(Line::from(vec![
         Span::styled(
             "❯ ",
             Style::default().fg(theme.accent).add_modifier(Modifier::BOLD),
         ),
         Span::styled(input, Style::default().fg(theme.foreground)),
         Span::styled(" ", Style::default()),
         Span::styled(
             status_text,
             Style::default().fg(theme.accent_light).add_modifier(Modifier::DIM),
         ),
     ]))
     .style(Style::default().fg(theme.foreground))
     .wrap(Wrap { trim: true })
 }
