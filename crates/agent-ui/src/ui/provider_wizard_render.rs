 use ratatui::layout::{Alignment, Constraint, Direction, Layout};
 use ratatui::style::{Color, Modifier, Style};
 use ratatui::widgets::{Block, BorderType, List, ListItem, Paragraph};
 
 use super::provider_wizard::{provider_name, ProviderWizardScreen, WizardStep};
 
 pub fn render_screen(wizard: &ProviderWizardScreen, frame: &mut ratatui::Frame, area: ratatui::layout::Rect) {
     let main_block = Block::bordered()
         .title("Provider Wizard - Configure your provider")
         .border_style(Style::default().fg(Color::Rgb(79, 193, 255)))
         .border_type(BorderType::Rounded);
 
     let inner_area = main_block.inner(area);
     frame.render_widget(main_block, area);
 
     let chunks = Layout::default()
         .direction(Direction::Vertical)
         .margin(2)
         .constraints([
             Constraint::Length(20),
             Constraint::Min(0),
             Constraint::Length(3),
         ])
         .split(inner_area);
 
     render_form(wizard, frame, chunks[0]);
     render_preview(wizard, frame, chunks[1]);
     render_status(frame, chunks[2]);
 }
 
 fn render_form(wizard: &ProviderWizardScreen, frame: &mut ratatui::Frame, area: ratatui::layout::Rect) {
     let block = Block::bordered()
         .title(step_title(wizard))
         .border_type(BorderType::Rounded)
         .style(Style::default().fg(Color::Rgb(213, 213, 213)));
 
     let inner = block.inner(area);
     frame.render_widget(block, area);
 
     let step_label = Paragraph::new(get_step_label(wizard))
         .style(Style::default().fg(Color::Rgb(156, 163, 175)));
 
     frame.render_widget(step_label, inner);
 }
 
 fn step_title(wizard: &ProviderWizardScreen) -> &'static str {
     match wizard.current_step {
         WizardStep::ProviderId => "Step 1: Provider ID",
         WizardStep::BaseUrl => "Step 2: Base URL",
         WizardStep::ApiKeyEnv => "Step 3: API Key Env",
         WizardStep::Check => "Step 4: Check",
     }
 }
 
 fn get_step_label(wizard: &ProviderWizardScreen) -> String {
     match wizard.current_step {
         WizardStep::ProviderId => {
             let selected = provider_name(&wizard.config.provider);
             format!("Provider: {selected} (Tab: change provider)")
         }
         WizardStep::BaseUrl => "Base URL:".to_string(),
         WizardStep::ApiKeyEnv => "API Key Env:".to_string(),
         WizardStep::Check => "Connection Check:".to_string(),
     }
 }
 
 fn render_preview(wizard: &ProviderWizardScreen, frame: &mut ratatui::Frame, area: ratatui::layout::Rect) {
     let block = Block::bordered()
         .title("Preview")
         .border_type(BorderType::Rounded)
         .style(
             Style::default()
                 .fg(Color::Rgb(156, 163, 175))
                 .add_modifier(Modifier::ITALIC),
         );
 
     let inner = block.inner(area);
 
     let mut lines = vec![
         format!("Provider: {}", provider_name(&wizard.config.provider)),
         format!(
             "Model: {}",
             if wizard.config.model.is_empty() { "-" } else { &wizard.config.model }
         ),
     ];
 
     if let Some(ref url) = wizard.config.base_url {
         lines.push(format!("Base URL: {url}"));
     }
 
     if let Some(ref key) = wizard.config.api_key_env {
         if !key.is_empty() {
             lines.push(format!("API Key Env: {key}"));
         }
     }
 
     let list_items: Vec<ListItem> = lines
         .iter()
         .map(|l| ListItem::new(l.as_str()).style(Style::default().fg(Color::Rgb(203, 213, 224))))
         .collect();
 
     let list = List::new(list_items).style(Style::default().fg(Color::Rgb(203, 213, 224)));
 
     frame.render_widget(block, area);
     frame.render_widget(list, inner);
 }
 
 fn render_status(frame: &mut ratatui::Frame, area: ratatui::layout::Rect) {
     let help_text = "Enter: next step | Backspace: previous step | Ctrl+S: save | Ctrl+C: cancel";
     let paragraph = Paragraph::new(help_text)
         .style(Style::default().fg(Color::Rgb(107, 114, 128)))
         .alignment(Alignment::Center);
 
     frame.render_widget(paragraph, area);
 }
