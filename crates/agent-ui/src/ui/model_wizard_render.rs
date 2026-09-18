use ratatui::layout::{Alignment, Constraint, Direction, Layout};
use ratatui::style::{Color, Modifier, Style};
use ratatui::widgets::{Block, BorderType, List, ListItem, Paragraph};

use super::model_wizard::{provider_name, ModelWizardScreen, WizardField};

pub fn render_screen(
    wizard: &ModelWizardScreen,
    frame: &mut ratatui::Frame,
    area: ratatui::layout::Rect,
) {
    let main_block = Block::bordered()
        .title("Model Wizard - Configure your AI model")
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

fn render_form(
    wizard: &ModelWizardScreen,
    frame: &mut ratatui::Frame,
    area: ratatui::layout::Rect,
) {
    let fields = [
        (WizardField::Provider, "Provider"),
        (WizardField::Model, "Model ID"),
        (WizardField::BaseUrl, "Base URL"),
        (WizardField::ApiKeyEnv, "API Key Env"),
        (WizardField::Temperature, "Temperature"),
        (WizardField::MaxTokens, "Max Tokens"),
    ];

    let chunk_height = area.height.saturating_sub(fields.len() as u16) as usize;
    let chunk_height = chunk_height.max(1);

    for (i, (field, label)) in fields.iter().enumerate() {
        let field_y_offset = (i * (chunk_height + 1)) as u16;
        let field_area = ratatui::layout::Rect {
            x: area.x,
            y: area.y.saturating_add(field_y_offset),
            width: area.width,
            height: chunk_height as u16,
        };

        let is_active = *field == wizard.current_field;
        let style = if is_active {
            Style::default()
                .fg(Color::Rgb(118, 158, 242))
                .add_modifier(Modifier::BOLD)
        } else {
            Style::default().fg(Color::Rgb(107, 114, 128))
        };

        let value = wizard.get_field_value(field);
        let placeholder = get_placeholder(field);

        let display_line = if is_active {
            format!("▶ {label}: {value} ({placeholder})")
        } else {
            format!("  {label}: {}", if value.is_empty() { "-" } else { &value })
        };

        let paragraph = Paragraph::new(display_line).style(style);
        let block = Block::bordered()
            .border_type(BorderType::Rounded)
            .style(style);
        let inner = block.inner(field_area);

        frame.render_widget(block, field_area);
        frame.render_widget(paragraph, inner);
    }
}

fn get_placeholder(field: &WizardField) -> &'static str {
    match field {
        WizardField::Provider => "Tab/Shift+Tab to change",
        WizardField::Model => "model name (e.g. gpt-4)",
        WizardField::BaseUrl => "http://... (optional)",
        WizardField::ApiKeyEnv => "e.g. OPENAI_API_KEY",
        WizardField::Temperature => "0.0-2.0",
        WizardField::MaxTokens => "e.g. 4096",
    }
}

fn render_preview(
    wizard: &ModelWizardScreen,
    frame: &mut ratatui::Frame,
    area: ratatui::layout::Rect,
) {
    let block = Block::bordered()
        .title("Preview")
        .border_type(BorderType::Rounded)
        .style(
            Style::default()
                .fg(Color::Rgb(156, 163, 175))
                .add_modifier(Modifier::ITALIC),
        );

    let inner = block.inner(area);

    let model_name = provider_name(&wizard.config.provider);
    let model_id = if wizard.config.model.is_empty() {
        "-"
    } else {
        &wizard.config.model
    };

    let mut lines = vec![
        format!("Provider: {model_name}"),
        format!("Model: {model_id}"),
    ];

    if let Some(ref url) = wizard.config.base_url {
        lines.push(format!("Base URL: {url}"));
    }

    if let Some(ref key) = wizard.config.api_key_env {
        lines.push(format!("API Key Env: {key}"));
    }

    if let Some(temp) = wizard.config.temperature {
        lines.push(format!("Temperature: {temp}"));
    }

    if let Some(tokens) = wizard.config.max_tokens {
        lines.push(format!("Max Tokens: {tokens}"));
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
    let help_text = "Tab: next field | Shift+Tab: prev field | Enter: set value | Ctrl+S: save | Ctrl+C: cancel";
    let paragraph = Paragraph::new(help_text)
        .style(Style::default().fg(Color::Rgb(107, 114, 128)))
        .alignment(Alignment::Center);

    frame.render_widget(paragraph, area);
}
