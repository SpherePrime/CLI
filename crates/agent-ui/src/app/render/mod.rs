mod chat;
mod input;
mod status;

use ratatui::layout::{Constraint, Direction, Layout};
use ratatui::prelude::*;

use super::state::AppState;
use super::AgentApp;
use crate::theme::Theme;

pub fn render_app(f: &mut Frame, _layout: Layout, app: &AgentApp) {
    let area = f.area();
    let theme = Theme::default_theme();

    match app.current_state {
        AppState::Banner => render_banner(f, area, app, &theme),
        AppState::Chat => render_chat(f, area, app, &theme),
        AppState::ModelWizard
        | AppState::ProviderWizard
        | AppState::ConfigMenu => render_wizard_status(f, area, app, &theme),
    }
}

fn render_chat(f: &mut Frame, area: Rect, app: &AgentApp, theme: &Theme) {
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(1),
            Constraint::Min(0),
            Constraint::Length(3),
        ])
        .split(area);

    status::render_status_bar(f, chunks[0], app, theme);
    chat::render_chat_area(f, chunks[1], app, theme);
    input::render_input_area(f, chunks[2], app, theme);
}

fn render_banner(f: &mut Frame, area: Rect, app: &AgentApp, _theme: &Theme) {
    let banner = crate::ui::banner::BannerScreen::new()
        .with_model(&app.model_config.model)
        .with_provider(super::provider_name(&app.model_config.provider))
        .with_version(&app.version);
    banner.render(f, area);
}

fn render_wizard_status(f: &mut Frame, area: Rect, app: &AgentApp, theme: &Theme) {
    let status_text = app.build_status_text();
    input::render_wizard_footer(f, area, app, &status_text, theme);
}

pub fn total_chat_height<L: IntoIterator<Item = crate::app::MessageEntry>>(
    _messages: L,
    _width: u16,
) -> usize {
    0
}